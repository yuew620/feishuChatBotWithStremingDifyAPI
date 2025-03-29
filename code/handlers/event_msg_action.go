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
	ctx, cancel := context.WithTimeout(*a.ctx, 60*time.Second)
	defer cancel()

	log.Printf("Processing message: %s from user: %s", a.info.qParsed, a.info.userId)

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

	// 发送处理中卡片并开始流式聊天
	cardCreateStart := time.Now()
	log.Printf("[Timing] 2. 开始创建卡片和发送AI请求: %v", cardCreateStart.Format("2006-01-02 15:04:05.000"))
	cardInfo, chatResponseStream, err := sendOnProcess(a, aiMessages)
	if err != nil {
		log.Printf("Failed to send processing card and start chat: %v", err)
		_ = sendMsg(*a.ctx, fmt.Sprintf("处理消息时出错: %v", err), a.info.chatId)
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

	// 设置整个流式处理的超时
	streamTimeout := time.NewTimer(55 * time.Second)
	defer streamTimeout.Stop()

	// 主循环处理响应
	streamingStartTime := time.Now()
	lastContentTime := time.Now()
	for {
		select {
		case err := <-errChan:
			errorMsg := "聊天失败"
			if err != nil {
				errorMsg = fmt.Sprintf("错误: %v", err)
			}
			log.Printf("Error received from errChan: %s", errorMsg)
			_ = updateFinalCard(ctx, errorMsg, cardInfo)
			return false

		case res, ok := <-chatResponseStream:
			if !ok {
				// 流结束，保存会话并更新最终卡片
				log.Printf("[Timing] Total streaming time: %v ms", time.Since(streamingStartTime).Milliseconds())
				if answer == "" {
					log.Printf("Warning: Received empty response from Dify")
					_ = updateFinalCard(ctx, "抱歉，未能获取到有效回复", cardInfo)
					return false
				}
				return m.handleCompletion(ctx, a, cardInfo, answer, aiMessages)
			}
			noContentTimeout.Stop()
			noContentTimeout.Reset(10 * time.Second)
			
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
			log.Printf("Received new content from stream: %s", res)
			log.Printf("Time since last content: %v ms", time.Since(lastContentTime).Milliseconds())
			lastContentTime = time.Now()
			
			// 直接在主线程中更新，确保顺序正确
			if err := updateTextCard(ctx, currentAnswer, cardInfo); err != nil {
				log.Printf("Failed to update card: %v", err)
			}
			log.Printf("[Timing] Card update took: %v ms", time.Since(updateStart).Milliseconds())
			
			// 不再添加延迟，让更新速度最大化
			m.mu.Unlock()

		case <-noContentTimeout.C:
			log.Printf("No content received for 10 seconds, timing out")
			_ = updateFinalCard(ctx, "请求超时，未收到响应", cardInfo)
			return false

		case <-streamTimeout.C:
			log.Printf("Stream processing timeout after 55 seconds")
			_ = updateFinalCard(ctx, "处理超时，请重试", cardInfo)
			return false

		case <-ctx.Done():
			log.Printf("Context deadline exceeded")
			_ = updateFinalCard(ctx, "请求超时", cardInfo)
			return false
		}
	}
}

func (m *MessageAction) handleCompletion(ctx context.Context, a *ActionInfo, cardInfo *CardInfo, answer string, aiMessages []ai.Message) bool {
	// 从session中获取会话信息
	sessionInfo, err := a.handler.sessionCache.GetSessionInfo(a.info.userId, *a.info.msgId)
	if err != nil {
		log.Printf("Failed to get session info: %v", err)
		return false
	}

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
		if err := a.handler.sessionCache.SetMessages(*a.info.sessionId, a.info.userId, aiMessages, sessionInfo.CardId, *a.info.msgId, sessionInfo.ConversationID, sessionInfo.CacheAddress); err != nil {
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

func sendOnProcess(a *ActionInfo, aiMessages []ai.Message) (*CardInfo, chan string, error) {
	log.Printf("Starting sendOnProcess for session %s", *a.info.sessionId)
	
	// 创建响应通道
	responseStream := make(chan string, 10)
	
	// 创建Dify消息处理函数
	difyHandler := func(ctx context.Context) error {
		log.Printf("Starting Dify handler for session %s", *a.info.sessionId)
		
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
		log.Printf("Sending StreamChat request to Dify for session %s", *a.info.sessionId)
		err := difyClient.StreamChat(ctx, difyMessages, responseStream)
		if err != nil {
			log.Printf("Error in Dify StreamChat for session %s: %v", *a.info.sessionId, err)
			return fmt.Errorf("failed to send message to Dify: %w", err)
		}
		
		log.Printf("Dify StreamChat completed successfully for session %s", *a.info.sessionId)
		return nil
	}
	
	// 使用并行处理函数
	log.Printf("Calling sendOnProcessCardAndDify for session %s", *a.info.sessionId)
	cardInfo, err := sendOnProcessCardAndDify(*a.ctx, a.info.sessionId, a.info.msgId, difyHandler)
	if err != nil {
		log.Printf("Error in sendOnProcessCardAndDify for session %s: %v", *a.info.sessionId, err)
		return nil, nil, fmt.Errorf("failed to send processing card: %w", err)
	}
	
	log.Printf("Processing card sent successfully for session %s, card ID: %s", *a.info.sessionId, cardInfo.CardId)

	// 创建一个新的通道来处理和记录从Dify接收到的消息
	processedStream := make(chan string, 10)
	go func() {
		defer close(processedStream)
		for msg := range responseStream {
			log.Printf("Received message from Dify for session %s: %s", *a.info.sessionId, msg)
			processedStream <- msg
		}
		log.Printf("Dify response stream closed for session %s", *a.info.sessionId)
	}()

	log.Printf("sendOnProcess completed for session %s", *a.info.sessionId)
	return cardInfo, processedStream, nil
}

// Add this new function to log the details of sendOnProcessCardAndDify
func sendOnProcessCardAndDify(ctx context.Context, sessionId, msgId *string, difyHandler func(context.Context) error) (*CardInfo, error) {
	log.Printf("Starting sendOnProcessCardAndDify for session %s", *sessionId)
	
	// Create processing card
	cardInfo, err := createProcessingCard(ctx, sessionId, msgId)
	if err != nil {
		log.Printf("Failed to create processing card for session %s: %v", *sessionId, err)
		return nil, fmt.Errorf("failed to create processing card: %w", err)
	}
	log.Printf("Processing card created successfully for session %s", *sessionId)

	// Start Dify chat in a separate goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Printf("Starting Dify handler goroutine for session %s", *sessionId)
		if err := difyHandler(ctx); err != nil {
			log.Printf("Dify handler error for session %s: %v", *sessionId, err)
			errChan <- err
		}
		close(errChan)
		log.Printf("Dify handler goroutine completed for session %s", *sessionId)
	}()

	// Wait for potential immediate errors
	select {
	case err := <-errChan:
		if err != nil {
			log.Printf("Immediate error from Dify handler for session %s: %v", *sessionId, err)
			return nil, fmt.Errorf("dify handler error: %w", err)
		}
	case <-time.After(100 * time.Millisecond):
		// No immediate error, continue
	}

	log.Printf("sendOnProcessCardAndDify completed successfully for session %s", *sessionId)
	return cardInfo, nil
}

// Add detailed logging to createProcessingCard function
func createProcessingCard(ctx context.Context, sessionId, msgId *string) (*CardInfo, error) {
	log.Printf("Starting createProcessingCard for session %s, message %s", *sessionId, *msgId)

	// Create the processing card
	cardInfo, err := createCard(ctx, sessionId, msgId, "正在处理中...")
	if err != nil {
		log.Printf("Failed to create card for session %s, message %s: %v", *sessionId, *msgId, err)
		return nil, fmt.Errorf("failed to create card: %w", err)
	}

	log.Printf("Card created successfully for session %s, message %s, card ID: %s", *sessionId, *msgId, cardInfo.CardId)

	// Store the card info in the session cache
	err = storeCardInfo(ctx, sessionId, msgId, cardInfo)
	if err != nil {
		log.Printf("Failed to store card info for session %s, message %s: %v", *sessionId, *msgId, err)
		return nil, fmt.Errorf("failed to store card info: %w", err)
	}

	log.Printf("Card info stored successfully for session %s, message %s", *sessionId, *msgId)

	return cardInfo, nil
}

// Add detailed logging to createCard function
func createCard(ctx context.Context, sessionId, msgId *string, content string) (*CardInfo, error) {
	log.Printf("Starting createCard for session %s, message %s", *sessionId, *msgId)

	// Create the card (implementation details may vary)
	cardInfo, err := cardCreator.CreateCard(ctx, content)
	if err != nil {
		log.Printf("Failed to create card for session %s, message %s: %v", *sessionId, *msgId, err)
		return nil, fmt.Errorf("failed to create card: %w", err)
	}

	log.Printf("Card created successfully for session %s, message %s, card ID: %s", *sessionId, *msgId, cardInfo.CardId)

	return cardInfo, nil
}

// Add detailed logging to storeCardInfo function
func storeCardInfo(ctx context.Context, sessionId, msgId *string, cardInfo *CardInfo) error {
	log.Printf("Starting storeCardInfo for session %s, message %s, card ID: %s", *sessionId, *msgId, cardInfo.CardId)

	// Store the card info in the session cache (implementation details may vary)
	err := sessionCache.StoreCardInfo(*sessionId, *msgId, cardInfo)
	if err != nil {
		log.Printf("Failed to store card info for session %s, message %s: %v", *sessionId, *msgId, err)
		return fmt.Errorf("failed to store card info: %w", err)
	}

	log.Printf("Card info stored successfully for session %s, message %s", *sessionId, *msgId)

	return nil
}
