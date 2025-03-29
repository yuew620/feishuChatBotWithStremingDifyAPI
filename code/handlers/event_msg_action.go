package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"start-feishubot/initialization"
	"start-feishubot/services/accesscontrol"
	"start-feishubot/services/ai"
	"start-feishubot/services/dify"
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
	startTime := time.Now()
	log.Printf("[Timing] ===== 新消息处理开始 =====")
	log.Printf("[Timing] 1. 收到用户消息时间: %v", startTime.Format("2006-01-02 15:04:05.000"))
	
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

	// 发送处理中卡片并开始流式聊天
	cardCreateStart := time.Now()
	log.Printf("[Timing] 2. 开始创建卡片和发送AI请求: %v", cardCreateStart.Format("2006-01-02 15:04:05.000"))
	cardInfo, chatResponseStream, err := sendOnProcess(a)
	if err != nil {
		log.Printf("Failed to send processing card and start chat: %v", err)
		return false
	}
	cardCreateEnd := time.Now()
	log.Printf("[Timing] 3. 卡片创建和AI请求发送完成: %v", cardCreateEnd.Format("2006-01-02 15:04:05.000"))
	log.Printf("[Timing] 卡片创建和AI请求发送总耗时: %v ms", time.Since(cardCreateStart).Milliseconds())

	errChan := make(chan error, 1)
	answer := ""
	
	// 设置无内容超时
	noContentTimeout := time.NewTimer(10 * time.Second)
	defer noContentTimeout.Stop()

	// 主循环处理响应
	for {
		select {
		case err := <-errChan:
			errorMsg := "聊天失败"
			if err != nil {
				errorMsg = fmt.Sprintf("错误: %v", err)
			}
			_ = updateFinalCard(ctx, errorMsg, cardInfo)
			return false

			case res, ok := <-chatResponseStream:
			if !ok {
				// 流结束，保存会话并更新最终卡片
				log.Printf("[Timing] Total streaming time: %v ms", time.Since(startTime).Milliseconds())
				return m.handleCompletion(ctx, a, cardInfo, answer, aiMessages)
			}
			noContentTimeout.Stop()
			
			m.mu.Lock()
			// 处理所有收到的内容，不再检查是否包含
			// 添加新内容到累积答案
			if answer == "" {
				answer = res
			} else {
				// 直接拼接内容，不添加额外空格
				answer = answer + res
			}
			
			// 使用流式更新API更新卡片内容
			currentAnswer := answer
			
			updateStart := time.Now()
			// 记录日志
			log.Printf("Updating card with new content: %s", res)
			
			// 直接在主线程中更新，确保顺序正确
			if err := updateTextCard(ctx, currentAnswer, cardInfo); err != nil {
				log.Printf("Failed to update card: %v", err)
			}
			log.Printf("[Timing] Card update took: %v ms", time.Since(updateStart).Milliseconds())
			
			// 不再添加延迟，让更新速度最大化
			m.mu.Unlock()

		case <-ctx.Done():
			_ = updateFinalCard(ctx, "请求超时", cardInfo)
			return false
		}
	}
}

func (m *MessageAction) handleCompletion(ctx context.Context, a *ActionInfo, cardInfo *CardInfo, answer string, aiMessages []ai.Message) bool {
	// 更新最终卡片
	if err := updateFinalCard(ctx, answer, cardInfo); err != nil {
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

func sendOnProcess(a *ActionInfo) (*CardInfo, chan string, error) {
	// 从会话缓存中获取历史消息
	aiMessages := a.handler.sessionCache.GetMessages(*a.info.sessionId)
	
	// 添加用户新消息，并设置元数据
	userMessage := ai.Message{
		Role:    "user",
		Content: a.info.qParsed,
		Metadata: map[string]string{
			"session_id": *a.info.sessionId,
			"user_id":    a.info.userId,
		},
	}
	aiMessages = append(aiMessages, userMessage)
	
	// 创建响应通道
	responseStream := make(chan string, 10)
	
	// 创建Dify消息处理函数
	difyHandler := func(ctx context.Context) error {
		// 预处理消息，准备发送到Dify
		difyMessages := make([]dify.Messages, len(aiMessages))
		for i, msg := range aiMessages {
			difyMessages[i] = dify.Messages{
				Role:     msg.Role,
				Content:  msg.Content,
				Metadata: msg.Metadata,
			}
		}
		
		// 发送请求到Dify服务
		difyClient := initialization.GetDifyClient()
		if err := difyClient.StreamChat(ctx, difyMessages, responseStream); err != nil {
			return fmt.Errorf("failed to send message to Dify: %w", err)
		}
		
		return nil
	}
	
	// 使用并行处理函数
	cardInfo, err := sendOnProcessCardAndDify(*a.ctx, a.info.sessionId, a.info.msgId, difyHandler)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send processing card: %w", err)
	}
	return cardInfo, responseStream, nil
}
