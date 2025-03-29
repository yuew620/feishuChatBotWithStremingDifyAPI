package handlers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	
	"start-feishubot/initialization"
	"start-feishubot/services/ai"
	"start-feishubot/services/dify"
)

// MessageBuffer 用于缓存消息内容
type MessageBuffer struct {
	content    string
	lastUpdate time.Time
	timer      *time.Timer
	mu         sync.Mutex
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
	if err := m.sessionCache.SetMessages(*a.info.sessionId, a.info.userId, aiMessages, cardInfo.CardEntityId, *a.info.msgId, sessionInfo.ConversationID, sessionInfo.CacheAddress); err != nil {
		log.Printf("Error saving session messages: %v", err)
	}

	log.Printf("Execute completed for session %s", *a.info.sessionId)
	return true
}

// sendOnProcess sends a processing card and starts the Dify chat
func (m *MessageHandler) sendOnProcess(a *ActionInfo, aiMessages []ai.Message) (*CardInfo, chan string, error) {
	log.Printf("Starting sendOnProcess for session %s", *a.info.sessionId)
	
	// Create a processing card
	cardInfo, err := m.getOrCreateCard(a.ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get or create card: %w", err)
	}

	// Create response channel and buffer
	responseStream := make(chan string, 10)
	messageBuffer := &MessageBuffer{
		content:    "",
		lastUpdate: time.Now(),
		timer:      time.NewTimer(300 * time.Millisecond),
	}

	// Get session info
	sessionInfo, _ := m.sessionCache.GetSessionInfo(a.info.userId, *a.info.msgId)
	var conversationID string
	if sessionInfo != nil {
		conversationID = sessionInfo.ConversationID
	}

	// Create Dify handler function
	difyHandler := func(ctx context.Context) error {
		log.Printf("Starting Dify handler for session %s", *a.info.sessionId)
		
		// Prepare messages for Dify
		difyMessages := make([]dify.Messages, len(aiMessages))
		for i, msg := range aiMessages {
			difyMessages[i] = dify.Messages{
				Role:     msg.Role,
				Content:  msg.Content,
				Metadata: msg.Metadata,
			}
		}
		
		// Send request to Dify service
		difyClient := initialization.GetDifyClient()
		log.Printf("Sending StreamChat request to Dify for session %s with conversationID: %s", *a.info.sessionId, conversationID)
		
		// Create context with timeout
		streamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Create message processing channel
		processedStream := make(chan string, 10)
		defer close(processedStream)

		var lastSendTime time.Time
		var lastContent string
		var cardSent bool

		// Start message processing goroutine
		go func() {
			for msg := range processedStream {
				messageBuffer.mu.Lock()
				messageBuffer.content += msg

				now := time.Now()
				shouldSend := false

				if !cardSent || now.Sub(lastSendTime) > 100*time.Millisecond {
					shouldSend = true
				}

				if shouldSend {
					if err := m.updateTextCard(*a.ctx, messageBuffer.content, cardInfo); err != nil {
						log.Printf("Error updating card: %v", err)
						if newCardInfo, err := m.getOrCreateCard(a.ctx); err == nil {
							cardInfo = newCardInfo
							if err := m.updateTextCard(*a.ctx, messageBuffer.content, cardInfo); err != nil {
								log.Printf("Error updating card with new card: %v", err)
								messageBuffer.mu.Unlock()
								continue
							}
						}
					}
					lastSendTime = now
					lastContent = messageBuffer.content
					cardSent = true
				}

				messageBuffer.timer.Reset(300 * time.Millisecond)
				messageBuffer.mu.Unlock()
			}

			// Final send
			messageBuffer.mu.Lock()
			if messageBuffer.content != "" && messageBuffer.content != lastContent {
				if err := m.updateTextCard(*a.ctx, messageBuffer.content, cardInfo); err != nil {
					log.Printf("Error updating final card: %v", err)
				}
			}
			messageBuffer.content = ""
			messageBuffer.mu.Unlock()
		}()

		// Start timer handler goroutine
		go func() {
			for {
				select {
				case <-messageBuffer.timer.C:
					messageBuffer.mu.Lock()
					if messageBuffer.content != "" && messageBuffer.content != lastContent {
						if err := m.updateTextCard(*a.ctx, messageBuffer.content, cardInfo); err != nil {
							log.Printf("Error updating card in timer: %v", err)
						} else {
							lastContent = messageBuffer.content
						}
						messageBuffer.content = ""
					}
					messageBuffer.mu.Unlock()
				case <-streamCtx.Done():
					return
				}
			}
		}()

		return difyClient.StreamChat(streamCtx, difyMessages, processedStream)
	}
	
	// Use parallel processing function
	log.Printf("Calling sendOnProcessCardAndDify for session %s", *a.info.sessionId)
	cardInfo, err = m.sendOnProcessCardAndDify(*a.ctx, a.info.sessionId, a.info.msgId, difyHandler)
	if err != nil {
		log.Printf("Error in sendOnProcessCardAndDify for session %s: %v", *a.info.sessionId, err)
		close(responseStream)
		return nil, nil, fmt.Errorf("failed to send processing card: %w", err)
	}
	
	log.Printf("Processing card sent successfully for session %s, card ID: %s", *a.info.sessionId, cardInfo.CardEntityId)

	// Create new channel to process and log messages from Dify
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

// getOrCreateCard gets a card from the pool or creates a new one
func (m *MessageHandler) getOrCreateCard(ctx *context.Context) (*CardInfo, error) {
	// Try to get card from pool
	cardPool := initialization.GetCardPool()
	if cardPool != nil {
		cardID, err := cardPool.GetCard(*ctx)
		if err == nil {
			log.Printf("Got card from pool: %s", cardID)
			return &CardInfo{
				CardEntityId: cardID,
				ElementId:    "content_block",
			}, nil
		}
		log.Printf("Failed to get card from pool: %v, creating new card", err)
	}

	// If pool is unavailable or get failed, create new card
	card, err := m.cardCreator.CreateCard(*ctx, "正在处理中...")
	if err != nil {
		return nil, fmt.Errorf("failed to create card: %w", err)
	}
	
	return &CardInfo{
		CardEntityId: card.CardId,
		ElementId:    "content_block",
	}, nil
}

// updateTextCard updates the card with new text content
func (m *MessageHandler) updateTextCard(ctx context.Context, msg string, cardInfo *CardInfo) error {
	return updateTextCard(ctx, msg, cardInfo)
}

// updateFinalCard updates the final state of the card
func (m *MessageHandler) updateFinalCard(ctx context.Context, msg string, cardInfo *CardInfo) error {
	return updateFinalCard(ctx, msg, cardInfo)
}

// sendOnProcessCardAndDify sends a processing card and handles Dify message processing
func (m *MessageHandler) sendOnProcessCardAndDify(ctx context.Context, sessionId *string, msgId *string, difyHandler func(context.Context) error) (*CardInfo, error) {
	return sendOnProcessCardAndDify(ctx, sessionId, msgId, difyHandler)
}
