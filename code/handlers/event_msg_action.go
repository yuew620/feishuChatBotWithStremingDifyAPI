package handlers

import (
	"context"
	"fmt"
	"log"
	"start-feishubot/initialization"
	"start-feishubot/services/ai"
	"start-feishubot/services/dify"
	"start-feishubot/services/cardcreator"
	"time"
)

import (
	"start-feishubot/services"
)

// MessageHandler handles the processing of messages
type MessageHandler struct {
	sessionCache services.SessionServiceCacheInterface
	cardCreator  cardcreator.CardCreator
}

// Execute processes the incoming message and manages the conversation flow
func (m *MessageHandler) Execute(a *ActionInfo) bool {
	log.Printf("Starting Execute for session %s", *a.info.sessionId)

	// Get historical messages
	aiMessages := m.sessionCache.GetMessages(*a.info.sessionId)

	// Add the new user message
	userMessage := ai.Message{
		Role:    "user",
		Content: a.info.qParsed,
		Metadata: map[string]string{
			"session_id": *a.info.sessionId,
			"user_id":    a.info.userId,
		},
	}
	aiMessages = append(aiMessages, userMessage)

	// Process the message and get the response
	cardInfo, processedStream, err := m.sendOnProcess(a, aiMessages)
	if err != nil {
		log.Printf("Error in sendOnProcess: %v", err)
		_ = m.updateFinalCard(*a.ctx, fmt.Sprintf("处理消息时出错: %v", err), cardInfo)
		return false
	}

	// Handle the processed stream
	answer := ""
	for msg := range processedStream {
		answer += msg
		if err := m.updateTextCard(*a.ctx, answer, cardInfo); err != nil {
			log.Printf("Error updating card: %v", err)
		}
	}

	// Final update
	if err := m.updateFinalCard(*a.ctx, answer, cardInfo); err != nil {
		log.Printf("Error updating final card: %v", err)
	}

	// Save the conversation
	aiMessages = append(aiMessages, ai.Message{Role: "assistant", Content: answer})
	sessionInfo, _ := m.sessionCache.GetSessionInfo(a.info.userId, *a.info.msgId)
	if err := m.sessionCache.SetMessages(*a.info.sessionId, a.info.userId, aiMessages, cardInfo.CardId, *a.info.msgId, sessionInfo.ConversationID, sessionInfo.CacheAddress); err != nil {
		log.Printf("Error saving session messages: %v", err)
	}

	log.Printf("Execute completed for session %s", *a.info.sessionId)
	return true
}

// sendOnProcessCardAndDify sends a processing card and starts the Dify chat
func (m *MessageHandler) sendOnProcessCardAndDify(ctx context.Context, sessionId, msgId *string, difyHandler func(context.Context) error) (*CardInfo, error) {
	log.Printf("Creating processing card for session %s", *sessionId)
	
	// Create a processing card using cardCreator
	card, err := m.cardCreator.CreateCard(ctx, "正在处理中...")
	if err != nil {
		log.Printf("Error creating processing card: %v", err)
		return nil, fmt.Errorf("failed to create processing card: %w", err)
	}
	
	cardInfo := &CardInfo{CardId: card.CardId}
	
	// Start the Dify chat in a goroutine
	go func() {
		if err := difyHandler(ctx); err != nil {
			log.Printf("Error in Dify handler: %v", err)
			// Here you might want to update the card with an error message
			_ = m.cardCreator.UpdateCard(ctx, card.CardId, "处理过程中出错: "+err.Error())
		}
	}()
	
	return cardInfo, nil
}

// updateTextCard updates the card with the given content
func (m *MessageHandler) updateTextCard(ctx context.Context, content string, cardInfo *CardInfo) error {
	log.Printf("Starting updateTextCard for card ID: %s", cardInfo.CardId)

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		err := m.cardCreator.UpdateCard(ctx, cardInfo.CardId, content)
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

// updateFinalCard updates the card with the final content
func (m *MessageHandler) updateFinalCard(ctx context.Context, content string, cardInfo *CardInfo) error {
	log.Printf("Updating final card for card ID: %s", cardInfo.CardId)
	return m.updateTextCard(ctx, content, cardInfo)
}

func (m *MessageHandler) sendOnProcess(a *ActionInfo, aiMessages []ai.Message) (*CardInfo, chan string, error) {
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
		
		streamStartTime := time.Now()
		
		// 创建一个带有超时的上下文
		streamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		
		errChan := make(chan error, 1)
		go func() {
			errChan <- difyClient.StreamChat(streamCtx, difyMessages, responseStream)
		}()
		
		select {
		case err := <-errChan:
			streamDuration := time.Since(streamStartTime)
			if err != nil {
				log.Printf("Error in Dify StreamChat for session %s: %v (duration: %v)", *a.info.sessionId, err, streamDuration)
				close(responseStream)
				return fmt.Errorf("failed to send message to Dify: %w", err)
			}
			log.Printf("Dify StreamChat completed successfully for session %s (duration: %v)", *a.info.sessionId, streamDuration)
			return nil
		case <-streamCtx.Done():
			streamDuration := time.Since(streamStartTime)
			log.Printf("Dify StreamChat timed out for session %s after %v", *a.info.sessionId, streamDuration)
			close(responseStream)
			return fmt.Errorf("Dify StreamChat timed out after %v", streamDuration)
		}
	}
	
	// 使用并行处理函数
	log.Printf("Calling sendOnProcessCardAndDify for session %s", *a.info.sessionId)
	cardInfo, err := m.sendOnProcessCardAndDify(*a.ctx, a.info.sessionId, a.info.msgId, difyHandler)
	if err != nil {
		log.Printf("Error in sendOnProcessCardAndDify for session %s: %v", *a.info.sessionId, err)
		close(responseStream)
		return nil, nil, fmt.Errorf("failed to send processing card: %w", err)
	}
	
	log.Printf("Processing card sent successfully for session %s, card ID: %s", *a.info.sessionId, cardInfo.CardId)

	// 创建一个新的通道来处理和记录从Dify接收到的消息
	processedStream := make(chan string, 10)
	go func() {
		defer close(processedStream)
		lastMessageTime := time.Now()
		for msg := range responseStream {
			currentTime := time.Now()
			timeSinceLastMessage := currentTime.Sub(lastMessageTime)
			log.Printf("Received message from Dify for session %s: %s (time since last message: %v)", *a.info.sessionId, msg, timeSinceLastMessage)
			processedStream <- msg
			lastMessageTime = currentTime
		}
		log.Printf("Dify response stream closed for session %s", *a.info.sessionId)
	}()

	log.Printf("sendOnProcess completed for session %s", *a.info.sessionId)
	return cardInfo, processedStream, nil
}
