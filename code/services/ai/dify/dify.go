package dify

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"start-feishubot/services/ai"
	"strings"
	"sync"
	"time"
)

type DifyProvider struct {
	config     ai.Config
	httpClient *http.Client
	mu         sync.RWMutex
}

// Dify API请求结构
type streamRequest struct {
	Query    string       `json:"query"`
	Messages []ai.Message `json:"messages,omitempty"`
	Stream   bool        `json:"stream"`
	User     string      `json:"user,omitempty"`    // 可选的用户标识
	Inputs   interface{} `json:"inputs,omitempty"`  // 可选的输入参数
}

// Dify API响应结构
type streamResponse struct {
	Event string `json:"event"`
	Data  struct {
		Text      string            `json:"text"`
		ErrorCode string            `json:"error_code,omitempty"`
		Error     string            `json:"error,omitempty"`
		Metadata  map[string]string `json:"metadata,omitempty"` // 元数据
	} `json:"data"`
}

// NewDifyProvider 创建Dify提供商实例
func NewDifyProvider(config ai.Config) *DifyProvider {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // 禁用压缩以避免流式传输问题
	}

	return &DifyProvider{
		config: config,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   config.GetTimeout(),
		},
	}
}

// StreamChat 实现Provider接口
func (d *DifyProvider) StreamChat(ctx context.Context, messages []ai.Message, responseStream chan string) error {
	// 验证消息
	if err := d.validateMessages(messages); err != nil {
		return err
	}

	// 构建请求体
	lastMsg := messages[len(messages)-1]
	historicalMessages := messages[:len(messages)-1]
	
	reqBody := streamRequest{
		Query:    lastMsg.Content,
		Messages: historicalMessages,
		Stream:   true,
	}

	// 使用重试机制发送请求
	var lastError error
	for retry := 0; retry <= d.config.GetMaxRetries(); retry++ {
		if retry > 0 {
			select {
			case <-ctx.Done():
				return ai.NewError(ai.ErrTimeout, "context cancelled during retry", ctx.Err())
			case <-time.After(time.Duration(retry) * time.Second):
			}
			log.Printf("Retrying request (attempt %d/%d)", retry+1, d.config.GetMaxRetries())
		}

		err := d.doStreamRequest(ctx, reqBody, responseStream)
		if err == nil {
			return nil
		}

		// 判断是否是临时错误
		if aiErr, ok := err.(*ai.Error); ok && !aiErr.IsTemporary() {
			return err
		}

		lastError = err
		log.Printf("Request failed (attempt %d/%d): %v", retry+1, d.config.GetMaxRetries(), err)
	}

	return ai.NewError(ai.ErrConnectionFailed, "max retries exceeded", lastError)
}

// Close 实现Provider接口
func (d *DifyProvider) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.httpClient.CloseIdleConnections()
	return nil
}

func (d *DifyProvider) validateMessages(messages []ai.Message) error {
	if len(messages) == 0 {
		return ai.NewError(ai.ErrInvalidMessage, "messages cannot be empty", nil)
	}

	for i, msg := range messages {
		if err := msg.Validate(); err != nil {
			return ai.NewError(ai.ErrInvalidMessage, 
				fmt.Sprintf("invalid message at index %d", i), err)
		}
	}

	return nil
}

func (d *DifyProvider) doStreamRequest(ctx context.Context, reqBody streamRequest, responseStream chan string) error {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return ai.NewError(ai.ErrInvalidMessage, "error marshaling request", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", 
		fmt.Sprintf("%s/v1/chat-messages", d.config.GetApiUrl()), 
		strings.NewReader(string(jsonBody)))
	if err != nil {
		return ai.NewError(ai.ErrConnectionFailed, "error creating request", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.config.GetApiKey()))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// 发送请求
	d.mu.RLock()
	resp, err := d.httpClient.Do(req)
	d.mu.RUnlock()

	if err != nil {
		if err == context.DeadlineExceeded {
			return ai.NewError(ai.ErrTimeout, "request timeout", err)
		}
		return ai.NewError(ai.ErrConnectionFailed, "error sending request", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ai.NewError(ai.ErrInvalidResponse, 
			fmt.Sprintf("unexpected status code: %d, body: %s", resp.StatusCode, string(body)), 
			nil)
	}

	// 处理流式响应
	reader := bufio.NewReader(resp.Body)
	buffer := make([]byte, 4096) // 增加缓冲区大小
	var partialLine string

	for {
		select {
		case <-ctx.Done():
			return ai.NewError(ai.ErrTimeout, "context cancelled", ctx.Err())
		default:
			n, err := reader.Read(buffer)
			if err != nil {
				if err == io.EOF {
					// 处理最后一行（如果有）
					if partialLine != "" {
						if err := d.processSSELine(partialLine, responseStream); err != nil {
							return err
						}
					}
					return nil
				}
				return ai.NewError(ai.ErrInvalidResponse, "error reading stream", err)
			}

			data := string(buffer[:n])
			lines := strings.Split(partialLine+data, "\n")
			
			// 处理完整的行
			for i := 0; i < len(lines)-1; i++ {
				line := strings.TrimSpace(lines[i])
				if line == "" {
					continue
				}
				if err := d.processSSELine(line, responseStream); err != nil {
					return err
				}
			}

			// 保存最后一个可能不完整的行
			partialLine = lines[len(lines)-1]
		}
	}
}

func (d *DifyProvider) processSSELine(line string, responseStream chan string) error {
	if !strings.HasPrefix(line, "data: ") {
		// 不是数据行，可能是注释或心跳
		return nil
	}

	data := strings.TrimPrefix(line, "data: ")
	var streamResp streamResponse
	if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
		// 尝试处理特殊格式
		if strings.Contains(data, "[DONE]") {
			return nil // 正常结束
		}
		return ai.NewError(ai.ErrInvalidResponse, "error unmarshaling response", err)
	}

	switch streamResp.Event {
	case "message":
		// 检查消息长度，避免超过飞书卡片限制
		if len(streamResp.Data.Text) > 0 {
			select {
			case responseStream <- streamResp.Data.Text:
			default:
				return ai.NewError(ai.ErrInvalidResponse, "response stream is blocked", nil)
			}
		}
	case "error":
		if streamResp.Data.ErrorCode != "" {
			return ai.NewError(ai.ErrInvalidResponse, 
				fmt.Sprintf("stream error: [%s] %s", 
					streamResp.Data.ErrorCode, streamResp.Data.Error), 
				nil)
		}
		return ai.NewError(ai.ErrInvalidResponse, 
			fmt.Sprintf("stream error: %s", streamResp.Data.Text), 
			nil)
	case "done":
		return nil
	case "ping":
		// 心跳事件，忽略
		return nil
	default:
		log.Printf("Unknown event type: %s", streamResp.Event)
		return nil // 不中断流处理
	}

	return nil
}

// DifyFactory 实现Factory接口
type DifyFactory struct{}

func (f *DifyFactory) CreateProvider(config ai.Config) (ai.Provider, error) {
	if config == nil {
		return nil, ai.NewError(ai.ErrInvalidConfig, "config cannot be nil", nil)
	}

	if config.GetProviderType() != string(ai.ProviderTypeDify) {
		return nil, ai.NewError(ai.ErrInvalidConfig, 
			fmt.Sprintf("invalid provider type: %s", config.GetProviderType()), 
			nil)
	}

	return NewDifyProvider(config), nil
}
