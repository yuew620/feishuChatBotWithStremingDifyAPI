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

	// 创建卡片实体
	cardIdStr, err := createCardEntity(*a.ctx, "正在思考中，请稍等...")
	if err != nil {
		log.Printf("Failed to create card entity: %v", err)
		return false
	}
	
	// 转换为指针类型
	cardId := &cardIdStr
	
	// 发送卡片实体
	_, err = sendCardEntity(*a.ctx, *cardId, *a.info.chatId)
	if err != nil {
		log.Printf("Failed to send card entity: %v", err)
		return false
	}
	
	// 记录日志
	log.Printf("Created and sent card entity with ID: %s", *cardId)

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
			// 只有新内容才处理
			if !strings.Contains(answer, res) {
				// 添加新内容到累积答案
				if answer == "" {
					answer = res
				} else {
					// 确保新内容是前一次更新的前缀
					// 不添加换行符，保持前缀关系
					answer = answer + " " + res
				}
				
				// 使用流式更新API更新卡片内容
				currentAnswer := answer
				
				// 记录日志
				log.Printf("Updating card with new content: %s", res)
				
				// 直接在主线程中更新，确保顺序正确
				if err := streamUpdateText(ctx, *cardId, "content_block", currentAnswer); err != nil {
					log.Printf("Failed to update card: %v", err)
				}
				
				// 添加小延迟，让打字机效果更明显
				time.Sleep(100 * time.Millisecond)
			}
			m.mu.Unlock()

		case <-ctx.Done():
			_ = updateFinalCard(ctx, "请求超时", cardId)
			return false
		}
	}
}

func (m *MessageAction) handleCompletion(ctx context.Context, a *ActionInfo, cardId *string, answer string, aiMessages []ai.Message) bool {
	// 更新最终卡片
	if err := streamUpdateText(ctx, *cardId, "content_block", answer); err != nil {
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
