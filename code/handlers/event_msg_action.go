package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/k0kubun/pp/v3"
	"log"
	"start-feishubot/initialization"
	"start-feishubot/services/accesscontrol"
	"start-feishubot/services/ai"
	"strings"
	"sync"
	"time"
)

type MessageAction struct {
	provider ai.Provider
	mu       sync.Mutex // 保护answer的并发访问
	// 活跃会话计数
	activeSessionsMu sync.RWMutex
	activeSessions  map[string]bool
}

func NewMessageAction(provider ai.Provider) *MessageAction {
	return &MessageAction{
		provider:       provider,
		activeSessions: make(map[string]bool),
	}
}

func (m *MessageAction) Execute(a *ActionInfo) bool {
	// 检查会话是否已经在处理中
	m.activeSessionsMu.Lock()
	if m.activeSessions[*a.info.sessionId] {
		m.activeSessionsMu.Unlock()
		log.Printf("Session %s is already being processed", *a.info.sessionId)
		_ = sendMsg(*a.ctx, "您的上一条消息正在处理中，请稍后再试", a.info.chatId)
		return false
	}
	m.activeSessions[*a.info.sessionId] = true
	m.activeSessionsMu.Unlock()

	// 确保在函数结束时清理会话状态
	defer func() {
		m.activeSessionsMu.Lock()
		delete(m.activeSessions, *a.info.sessionId)
		m.activeSessionsMu.Unlock()
	}()

	// Add access control
	if initialization.GetConfig().AccessControlEnable &&
		!accesscontrol.CheckAllowAccessThenIncrement(&a.info.userId) {

		msg := fmt.Sprintf("UserId: 【%s】 has accessed max count today! Max access count today %s: 【%d】",
			a.info.userId, accesscontrol.GetCurrentDateFlag(), initialization.GetConfig().AccessControlMaxCountPerUserPerDay)

		_ = sendMsg(*a.ctx, msg, a.info.chatId)
		return false
	}

	// 创建一个新的context，用于整个请求的生命周期
	ctx, cancel := context.WithTimeout(*a.ctx, 30*time.Second)
	defer cancel()

	log.Printf("Processing message: %s from user: %s", a.info.qParsed, a.info.userId)

	cardId, err := sendOnProcess(a)
	if err != nil {
		log.Printf("Failed to send processing card: %v", err)
		return false
	}

	answer := ""
	chatResponseStream := make(chan string, 100) // 缓冲区避免阻塞
	done := make(chan struct{})
	
	// 创建错误通道
	errChan := make(chan error, 1)

	// 设置无响应超时
	noContentTimeout := time.AfterFunc(10*time.Second, func() {
		pp.Println("no content timeout")
		select {
		case errChan <- fmt.Errorf("no content timeout"):
		default:
		}
	})
	defer noContentTimeout.Stop()

	// 获取并验证会话历史
	messages := a.handler.sessionCache.GetMessages(*a.info.sessionId)
	log.Printf("Retrieved %d historical messages for session: %s", len(messages), *a.info.sessionId)
	
	aiMessages := make([]ai.Message, 0, len(messages)+1)
	
	// 转换并验证历史消息
	for _, m := range messages {
		if err := m.Validate(); err != nil {
			log.Printf("Invalid historical message: %v", err)
			continue
		}
		aiMessages = append(aiMessages, m)
	}

	// 添加并验证用户新消息
	newMsg := ai.Message{
		Role:    "user",
		Content: a.info.qParsed,
	}
	if err := newMsg.Validate(); err != nil {
		_ = updateFinalCard(ctx, "消息格式错误", cardId)
		return false
	}
	aiMessages = append(aiMessages, newMsg)

	// 启动AI对话协程
	go func() {
		defer close(done)
		defer close(chatResponseStream)

	log.Printf("Sending request to AI provider with %d messages", len(aiMessages))
	if err := m.provider.StreamChat(ctx, aiMessages, chatResponseStream); err != nil {
		log.Printf("AI provider error: %v", err)
		select {
		case errChan <- err:
		default:
		}
		return
	}
	}()

	// 启动卡片更新协程
	ticker := time.NewTicker(300 * time.Millisecond)  // 更新频率提高到300ms
	defer ticker.Stop()

	var updateBuffer strings.Builder  // 用于缓存更新内容
	lastUpdate := time.Now()

	updateChan := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				select {
				case updateChan <- struct{}{}:
				default:
				}
			}
		}
	}()

	// 主循环处理响应
	for {
		select {
		case err := <-errChan:
			errorMsg := "聊天失败"
			if err != nil {
				errorMsg = fmt.Sprintf("错误: %v", err)
			}
			_ = updateFinalCard(ctx, errorMsg, cardId)
			return false

		case res, ok := <-chatResponseStream:
			if !ok {
				// 流结束，保存会话并更新最终卡片
				return m.handleCompletion(ctx, a, cardId, answer, aiMessages)
			}
			noContentTimeout.Stop()
			m.mu.Lock()
			// Only append if it's new content
			if !strings.Contains(answer, res) {
				if answer != "" {
					answer += "\n"  // Add newline between responses
				}
				answer += res
				updateBuffer.WriteString(res)
				
				// 如果距离上次更新超过100ms或缓冲区超过50字符，就更新卡片
				if time.Since(lastUpdate) > 100*time.Millisecond || updateBuffer.Len() > 50 {
					currentAnswer := answer
					updateBuffer.Reset()
					lastUpdate = time.Now()
					
					// 异步更新卡片，避免阻塞主流程
					go func(content string) {
						if err := updateTextCard(ctx, content, cardId); err != nil {
							log.Printf("Failed to update card: %v", err)
						}
					}(currentAnswer)
				}
			} else {
				log.Printf("Skipping duplicate content in card update: %s", res)
			}
			m.mu.Unlock()

		case <-updateChan:
			// No need to update here since we're updating immediately when receiving content
			continue

		case <-ctx.Done():
			_ = updateFinalCard(ctx, "请求超时", cardId)
			return false
		}
	}
}

func (m *MessageAction) handleCompletion(ctx context.Context, a *ActionInfo, cardId *string, answer string, aiMessages []ai.Message) bool {
	// 更新最终卡片
	if err := updateFinalCard(ctx, answer, cardId); err != nil {
		log.Printf("Failed to update final card: %v", err)
		return false
	}

	// 添加AI回复到会话历史
	if answer != "" {
		aiMessages = append(aiMessages, ai.Message{
			Role:    "assistant",
			Content: answer,
		})

		// 保存会话消息
		if err := a.handler.sessionCache.SetMessages(*a.info.sessionId, a.info.userId, aiMessages); err != nil {
			log.Printf("Failed to save session messages: %v", err)
		}
	} else {
		log.Printf("Empty response from AI provider, not saving to session history")
		return false
	}

	// 记录成功日志
	jsonByteArray, err := json.Marshal(aiMessages)
	if err != nil {
		log.Printf("Error marshaling messages: %v", err)
	} else {
		jsonStr := strings.ReplaceAll(string(jsonByteArray), "\\n", "")
		jsonStr = strings.ReplaceAll(jsonStr, "\n", "")
		log.Printf("\nSuccess request: UserId: %s\nMessages: %s\nFinal Response: %s\n",
			a.info.userId, jsonStr, answer)
	}

	return false
}

func printErrorMessage(a *ActionInfo, msg []ai.Message, err error) {
	log.Printf("Failed request: UserId: %s , Request: %s , Err: %s", a.info.userId, msg, err)
}

func sendOnProcess(a *ActionInfo) (*string, error) {
	cardId, err := sendOnProcessCard(*a.ctx, a.info.sessionId, a.info.msgId)
	if err != nil {
		return nil, fmt.Errorf("failed to send processing card: %w", err)
	}
	return cardId, nil
}
