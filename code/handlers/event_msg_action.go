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
			
			// 使用新的updateTextCard函数更新卡片
			updateErr := updateTextCard(ctx, currentAnswer, cardInfo)
			if updateErr != nil {
				log.Printf("Failed to update card: %v", updateErr)
				// 如果更新失败，我们仍然继续处理，但记录错误
				select {
				case errChan <- fmt.Errorf("card update failed: %w", updateErr):
				default:
					// 如果errChan已满，记录错误但不阻塞
					log.Printf("Error channel full, logging card update error: %v", updateErr)
				}
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

func updateTextCard(ctx context.Context, content string, cardInfo *CardInfo) error {
	log.Printf("Starting updateTextCard for card ID: %s", cardInfo.CardId)

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		err := cardService.UpdateCard(ctx, cardInfo.CardId, content)
		if err == nil {
			log.Printf("Card update successful for card ID: %s", cardInfo.CardId)
			return nil
		}

		log.Printf("Attempt %d failed to update card ID %s: %v", i+1, cardInfo.CardId, err)

		if i < maxRetries-1 {
			// Wait for a short duration before retrying
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while retrying card update: %w", ctx.Err())
			case <-time.After(time.Duration(i+1) * 100 * time.Millisecond):
				// Exponential backoff
			}
		}
	}

	return fmt.Errorf("failed to update card after %d attempts", maxRetries)
}
