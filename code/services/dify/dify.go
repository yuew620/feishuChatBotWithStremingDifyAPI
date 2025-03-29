package dify

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 会话缓存条目
type conversationEntry struct {
	conversationID string
	timestamp      time.Time
}

type DifyClient struct {
	config Config
	
	// 用户ID到Dify conversation ID的映射
	conversationsMu sync.RWMutex
	conversations   map[string]conversationEntry // userID -> {conversationId, timestamp}
}

type Messages struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"` // 添加元数据字段
}

type StreamRequest struct {
	Inputs          map[string]string `json:"inputs"`
	Query           string            `json:"query"`
	ResponseMode    string            `json:"response_mode"`
	ConversationId  string            `json:"conversation_id"`
	User            string            `json:"user"`
}

type StreamResponse struct {
	Event string `json:"event"`
	ConversationId string `json:"conversation_id,omitempty"` // 添加会话ID字段
	Data  struct {
		Text string `json:"text"`
		ConversationId string `json:"conversation_id,omitempty"` // 有时会在data中返回会话ID
	} `json:"data"`
}

// mustMarshal 将数据序列化为JSON，如果出错则panic
func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("json marshal error: %v", err))
	}
	return data
}

func NewDifyClient(config Config) *DifyClient {
	client := &DifyClient{
		config: config,
		conversations: make(map[string]conversationEntry),
	}
	
	// 启动一个后台goroutine，定期清理过期的会话缓存
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // 每小时检查一次
		defer ticker.Stop()
		
		for range ticker.C {
			client.cleanupConversations()
		}
	}()
	
	return client
}

// cleanupConversations 清理超过2小时的会话缓存
func (d *DifyClient) cleanupConversations() {
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

func (d *DifyClient) StreamChat(ctx context.Context, messages []Messages, responseStream chan string) error {
	defer close(responseStream) // Ensure the channel is closed when the function exits

	log.Printf("Starting StreamChat for user %s", messages[len(messages)-1].Metadata["user_id"])

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

	reqBody := StreamRequest{
		Inputs: map[string]string{
			"history": string(mustMarshal(messageHistory)),
		},
		Query:           lastMsg.Content,
		ResponseMode:    "streaming",
		ConversationId:  conversationID,
		User:            userID,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling request: %v", err)
		return fmt.Errorf("error marshaling request: %v", err)
	}

	// 构建完整的API URL（处理末尾斜杠）
	baseUrl := strings.TrimRight(d.config.GetDifyApiUrl(), "/")
	apiUrl := fmt.Sprintf("%s/v1/chat-messages", baseUrl)

	// 打印请求详情
	log.Printf("Sending request to Dify:\nURL: %s\nHeaders: %v\nBody: %s\n", 
		apiUrl,
		map[string]string{
			"Content-Type": "application/json",
			"Authorization": fmt.Sprintf("Bearer %s", d.config.GetDifyApiKey()),
		},
		string(jsonBody))

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", 
		apiUrl, 
		strings.NewReader(string(jsonBody)))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return fmt.Errorf("error creating request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.config.GetDifyApiKey()))

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request: %v", err)
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// 打印响应状态码和头部
	log.Printf("Dify response:\nStatus: %s\nHeaders: %v\n", 
		resp.Status, 
		resp.Header)

	// 如果状态码不是200，读取并打印错误响应
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Unexpected status code: %d, body: %s", resp.StatusCode, string(body))
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// 处理流式响应
	reader := bufio.NewReader(resp.Body)
	idleTimer := time.NewTimer(5 * time.Second)
	defer idleTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Context cancelled, stopping stream processing")
			return ctx.Err()
		case <-idleTimer.C:
			log.Printf("Stream has been idle for 5 seconds")
			return fmt.Errorf("stream idle timeout")
		default:
			idleTimer.Reset(5 * time.Second)
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					log.Printf("Reached end of stream")
					return nil
				}
				log.Printf("Error reading stream: %v", err)
				return fmt.Errorf("error reading stream: %v", err)
			}

			if strings.TrimSpace(line) == "" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				log.Printf("Received non-data line: %s", line)
				continue
			}

			// 解析SSE数据
			data := strings.TrimPrefix(line, "data: ")
			var streamResp StreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				// 尝试处理特殊格式
				if strings.Contains(data, "[DONE]") {
					log.Printf("Received [DONE] signal, ending stream")
					return nil // 正常结束
				}
				log.Printf("Error unmarshaling response: %v, data: %s", err, data)
				continue // 继续处理其他行
			}
			
			// 提取conversation_id并存储到缓存中
			if userID != "" {
				// 首先检查响应中是否包含conversation_id
				respConversationID := streamResp.ConversationId
				if respConversationID == "" {
					respConversationID = streamResp.Data.ConversationId
				}
				
				// 如果找到了conversation_id，存储到缓存中
				if respConversationID != "" && respConversationID != conversationID {
					d.conversationsMu.Lock()
					d.conversations[userID] = conversationEntry{
						conversationID: respConversationID,
						timestamp:      time.Now(),
					}
					d.conversationsMu.Unlock()
					log.Printf("Stored conversation_id %s for user %s", respConversationID, userID)
					
					// 更新当前使用的conversation_id
					conversationID = respConversationID
				}
			}

			// 处理不同的事件类型
			switch streamResp.Event {
			case "message":
				log.Printf("Received message event: %s", streamResp.Data.Text)
				responseStream <- streamResp.Data.Text
			case "error":
				log.Printf("Received error event: %s", streamResp.Data.Text)
				return fmt.Errorf("stream error: %s", streamResp.Data.Text)
			case "done":
				log.Printf("Received done event")
				return nil
			default:
				log.Printf("Received unknown event type: %s", streamResp.Event)
			}
		}
	}
}
