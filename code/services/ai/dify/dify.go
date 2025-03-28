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

type conversationEntry struct {
	conversationID string
	timestamp      time.Time
}

type DifyProvider struct {
	config     ai.Config
	httpClient *http.Client
	mu         sync.RWMutex
	sentContent map[string]bool  // Track content we've already sent
	
	// 会话ID到Dify conversation ID的映射
	conversationsMu sync.RWMutex
	conversations   map[string]conversationEntry // sessionId -> {conversationId, timestamp}
	
	// 用于累积内容的缓冲区
	bufferMu      sync.Mutex
	buffer        string
	lastSendTime  time.Time
}

// Dify API请求结构
type streamRequest struct {
	Inputs          map[string]string `json:"inputs"`
	Query           string            `json:"query"`
	ResponseMode    string            `json:"response_mode"`
	ConversationId  string            `json:"conversation_id"`
	User            string            `json:"user"`
}

// Dify API响应结构
type streamResponse struct {
	Event           string            `json:"event"`
	Thought         string            `json:"thought,omitempty"`    // agent_thought events use this field
	ConversationId  string            `json:"conversation_id,omitempty"` // 会话ID
	Answer          string            `json:"answer,omitempty"`     // agent_message events use this field
	Data       struct {
		Text          string            `json:"text"`
		Answer        string            `json:"answer,omitempty"`  // Some events use answer field
		Message       string            `json:"message,omitempty"` // Some events use message field
		ErrorCode     string            `json:"error_code,omitempty"`
		Error         string            `json:"error,omitempty"`
		Metadata      map[string]string `json:"metadata,omitempty"` // 元数据
		ConversationId string            `json:"conversation_id,omitempty"` // 有时会在data中返回会话ID
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

	provider := &DifyProvider{
		config: config,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   config.GetTimeout(),
		},
		sentContent: make(map[string]bool),
		conversations: make(map[string]conversationEntry),
		buffer: "",
		lastSendTime: time.Now(),
	}
	
	// 启动一个后台goroutine，定期清理过期的会话缓存
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // 每小时检查一次
		defer ticker.Stop()
		
		for range ticker.C {
			provider.cleanupConversations()
		}
	}()
	
	return provider
}

// cleanupConversations 清理超过2小时的会话缓存
func (d *DifyProvider) cleanupConversations() {
	d.conversationsMu.Lock()
	defer d.conversationsMu.Unlock()
	
	now := time.Now()
	expiredTime := now.Add(-2 * time.Hour) // 2小时过期
	
	// 遍历所有会话，删除过期的
	for userID, entry := range d.conversations {
		if entry.timestamp.Before(expiredTime) {
			delete(d.conversations, userID)
			log.Printf("Cleaned up expired conversation for user %s", userID)
		}
	}
	
	log.Printf("Conversation cache cleanup completed, remaining entries: %d", len(d.conversations))
}

// StreamChat 实现Provider接口
func (d *DifyProvider) StreamChat(ctx context.Context, messages []ai.Message, responseStream chan string) error {
	// Clear sent content map at the start of each chat
	d.mu.Lock()
	d.sentContent = make(map[string]bool)
	d.mu.Unlock()

	// 验证消息
	if err := d.validateMessages(messages); err != nil {
		return err
	}

	// 构建请求体
	lastMsg := messages[len(messages)-1]
	historicalMessages := messages[:len(messages)-1]
	
	// 构建消息历史
	var messageHistory []map[string]string
	for _, msg := range historicalMessages {
		messageHistory = append(messageHistory, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	var historyStr string
	if len(messageHistory) > 0 {
		historyJSON, err := json.Marshal(messageHistory)
		if err != nil {
			return ai.NewError(ai.ErrInvalidMessage, "error marshaling message history", err)
		}
		historyStr = string(historyJSON)
	} else {
		historyStr = "[]"  // Empty array for no history
	}

	// 从最后一条消息中提取用户ID
	userID := "feishu-bot" // 默认值
	if lastMsg.Metadata != nil {
		if id, ok := lastMsg.Metadata["user_id"]; ok && id != "" {
			userID = id
			log.Printf("Using user_id from metadata: %s", userID)
		}
	}
	
	// 检查是否有缓存的conversation_id
	conversationID := ""
	if userID != "" {
		// 从缓存中获取conversation_id
		d.conversationsMu.RLock()
		if entry, ok := d.conversations[userID]; ok {
			conversationID = entry.conversationID
			log.Printf("Using cached conversation_id for user %s: %s", userID, conversationID)
		}
		d.conversationsMu.RUnlock()
	}
	
	reqBody := streamRequest{
		Inputs: map[string]string{
			"history": historyStr,
		},
		Query:           lastMsg.Content,
		ResponseMode:    "streaming",
		ConversationId:  conversationID,
		User:            userID,
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

		// 创建一个新的上下文，包含用户ID
		ctxWithSessionID := context.WithValue(ctx, "userID", userID)
		err := d.doStreamRequest(ctxWithSessionID, reqBody, responseStream)
		if err == nil {
			return nil
		}

		// 检查是否是"Conversation Not Exists"错误
		if strings.Contains(err.Error(), "Conversation Not Exists") {
			log.Printf("Conversation not found, retrying without conversation_id")
			// 清除conversation_id并重试
			reqBody.ConversationId = ""
			err = d.doStreamRequest(ctxWithSessionID, reqBody, responseStream)
			if err == nil {
				return nil
			}
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
	// Ensure API URL doesn't end with slash
	apiURL := strings.TrimRight(d.config.GetApiUrl(), "/")
	fullURL := fmt.Sprintf("%s/v1/chat-messages", apiURL)
	
	log.Printf("Making request to Dify API: %s", fullURL)
	log.Printf("Request body: %s", string(jsonBody))
	
	req, err := http.NewRequestWithContext(ctx, "POST", 
		fullURL,
		strings.NewReader(string(jsonBody)))
	if err != nil {
		return ai.NewError(ai.ErrConnectionFailed, "error creating request", err)
	}

	// 设置所有请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	
	// 智能处理API key格式
	apiKey := d.config.GetApiKey()
	if !strings.HasPrefix(apiKey, "Bearer ") && !strings.HasPrefix(apiKey, "bearer ") {
		apiKey = "Bearer " + apiKey
	}
	req.Header.Set("Authorization", apiKey)
	
	// 记录完整的请求信息
	log.Printf("Request headers: Authorization: %s...", apiKey[:10])
	log.Printf("Full request URL: %s", fullURL)
	log.Printf("Full request headers: Content-Type: %s, Accept: %s, Cache-Control: %s", 
		req.Header.Get("Content-Type"), 
		req.Header.Get("Accept"),
		req.Header.Get("Cache-Control"))

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
		log.Printf("Dify API error response: Status: %d, Body: %s", resp.StatusCode, string(body))
		return ai.NewError(ai.ErrInvalidResponse, 
			fmt.Sprintf("unexpected status code: %d, body: %s", resp.StatusCode, string(body)), 
			nil)
	}
	
	log.Printf("Successfully connected to Dify API, starting to process stream")

	// 处理流式响应
	reader := bufio.NewReader(resp.Body)
	buffer := make([]byte, 512) // 进一步减小缓冲区大小以获得更频繁的更新
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
						if err := d.processSSELine(partialLine, responseStream, ctx); err != nil {
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
				if err := d.processSSELine(line, responseStream, ctx); err != nil {
					return err
				}
			}

			// 保存最后一个可能不完整的行
			partialLine = lines[len(lines)-1]
		}
	}
}

func (d *DifyProvider) processSSELine(line string, responseStream chan string, ctx context.Context) error {
	// 从上下文中提取用户ID，用于存储conversation_id
	userID, _ := ctx.Value("userID").(string)
	if !strings.HasPrefix(line, "data: ") {
		// 不是数据行，可能是注释或心跳
		return nil
	}

	data := strings.TrimPrefix(line, "data: ")
	
	// Log raw SSE data for debugging
	log.Printf("Raw SSE data: %s", data)
	
	var streamResp streamResponse
	if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
		// 尝试处理特殊格式
		if strings.Contains(data, "[DONE]") {
			return nil // 正常结束
		}
		log.Printf("Error unmarshaling response: %v, data: %s", err, data)
		return ai.NewError(ai.ErrInvalidResponse, "error unmarshaling response", err)
	}

	// Log the event type and content details
	log.Printf("Processing SSE event: %s", streamResp.Event)
	if streamResp.Event == "message" || streamResp.Event == "agent_message" {
		log.Printf("Message content - Text: %s, Answer: %s, Message: %s, TopLevelAnswer: %s", 
			streamResp.Data.Text, streamResp.Data.Answer, streamResp.Data.Message, streamResp.Answer)
	} else if streamResp.Event == "agent_thought" {
		log.Printf("Thought content: %s", streamResp.Thought)
	}
	
	// 提取conversation_id并存储到缓存中
	if userID != "" {
		// 首先检查响应中是否包含conversation_id
		conversationID := streamResp.ConversationId
		if conversationID == "" {
			conversationID = streamResp.Data.ConversationId
		}
		
		// 如果找到了conversation_id，存储到缓存中
		if conversationID != "" {
			d.conversationsMu.Lock()
			d.conversations[userID] = conversationEntry{
				conversationID: conversationID,
				timestamp:      time.Now(),
			}
			d.conversationsMu.Unlock()
			log.Printf("Stored conversation_id %s for user %s", conversationID, userID)
		}
	}

	switch streamResp.Event {
	case "message", "agent_message":
		// 对于 agent_message 事件，优先使用顶级的 Answer 字段
		var content string
		if streamResp.Event == "agent_message" && streamResp.Answer != "" {
			content = streamResp.Answer
			log.Printf("Using top-level Answer field: %s", content)
		} else {
			// 尝试其他可能的内容字段
			content = streamResp.Data.Text
			if content == "" {
				content = streamResp.Data.Answer
			}
			if content == "" {
				content = streamResp.Data.Message
			}
		}

		// 检查消息长度，避免超过飞书卡片限制
		if len(content) > 0 {
			if d.sentContent[content] {
				log.Printf("Skipping duplicate content: %s", content)
			} else {
				log.Printf("Adding content to buffer: %s", content)
				d.sentContent[content] = true
				
				// 使用缓冲区累积内容并定期发送
				if err := d.addToBufferAndSend(content, responseStream, ctx); err != nil {
					return err
				}
			}
		}
	case "agent_thought":
		// Handle agent_thought event specifically
		if streamResp.Thought != "" {
			if d.sentContent[streamResp.Thought] {
				log.Printf("Skipping duplicate thought: %s", streamResp.Thought)
			} else {
				log.Printf("Sending new thought to response stream: %s", streamResp.Thought)
				d.sentContent[streamResp.Thought] = true
				select {
				case responseStream <- streamResp.Thought:
				default:
					return ai.NewError(ai.ErrInvalidResponse, "response stream is blocked", nil)
				}
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
	case "done", "message_end":
		return nil
	case "ping":
		// 忽略心跳事件
		return nil
	default:
		log.Printf("Unknown event type: %s with text: %s", streamResp.Event, streamResp.Data.Text)
		return nil // 不中断流处理
	}

	return nil
}

// addToBufferAndSend 将内容添加到缓冲区，并在适当的时候发送到响应流
func (d *DifyProvider) addToBufferAndSend(content string, responseStream chan string, ctx context.Context) error {
	d.bufferMu.Lock()
	defer d.bufferMu.Unlock()
	
	// 添加内容到缓冲区
	if d.buffer == "" {
		d.buffer = content
	} else {
		d.buffer = d.buffer + content
	}
	
	// 检查是否应该发送缓冲区内容
	now := time.Now()
	if now.Sub(d.lastSendTime) >= 20*time.Millisecond {
		// 已经过了20ms，发送缓冲区内容
		if d.buffer != "" {
			log.Printf("Sending buffered content to response stream: %s", d.buffer)
			select {
			case responseStream <- d.buffer:
				// 发送成功，清空缓冲区并更新最后发送时间
				d.buffer = ""
				d.lastSendTime = now
			default:
				return ai.NewError(ai.ErrInvalidResponse, "response stream is blocked", nil)
			}
		}
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
