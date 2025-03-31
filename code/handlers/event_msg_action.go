package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/services/ai"
)

type MessageEventHandler struct {
	ctx     *context.Context
	info    *MsgInfo
	handler *MessageHandler
}

func (m *MessageEventHandler) Execute(info *ActionInfo) bool {
	if !m.handler.msgCache.IfProcessed(*m.info.msgId) {
		m.handler.msgCache.TagProcessed(*m.info.msgId)
		return true
	}
	return false
}

func NewMessageEventHandler(ctx *context.Context, info *MsgInfo, handler *MessageHandler) *MessageEventHandler {
	return &MessageEventHandler{
		ctx:     ctx,
		info:    info,
		handler: handler,
	}
}

func handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1, handler *MessageHandler) error {
	info := NewMsgInfo(event)

	// Create action info
	actionInfo := NewActionInfo(&ctx, info, handler)

	// Create message handler
	msgHandler := NewMessageEventHandler(&ctx, info, handler)

	// Execute handler
	if !msgHandler.Execute(actionInfo) {
		return nil
	}

	// Get message content
	content := *event.Event.Message.Content

	// Parse content
	var msg struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &msg); err != nil {
		return err
	}

	// Create AI messages
	messages := []ai.Message{
		{
			Role:    "user",
			Content: msg.Text,
		},
	}

	// Get response stream
	responseStream := make(chan string)
	defer close(responseStream)

	// Create context with timeout for AI request
	aiCtx, aiCancel := context.WithTimeout(ctx, 30*time.Second)
	defer aiCancel()

	// Get AI provider
	aiProvider := handler.dify

	// Get initial card from pool
	log.Printf("Getting card from pool")
	cardCtx, cardCancel := context.WithTimeout(ctx, 10*time.Second)
	cardID, err := handler.cardPool.GetCard(cardCtx)
	cardCancel()
	if err != nil {
		log.Printf("Failed to get card from pool: %v", err)
		return fmt.Errorf("failed to get card from pool: %v", err)
	}
	log.Printf("Got card from pool: %s", cardID)

	// Update card with initial "processing" message
	updateCtx, updateCancel := context.WithTimeout(ctx, 10*time.Second)
	_, err = handler.cardCreator.UpdateCardContent(updateCtx, cardID, "正在处理...")
	updateCancel()
	if err != nil {
		log.Printf("Failed to update card with processing message: %v", err)
		return err
	}

	// Stream chat
	streamDone := make(chan error)
	go func() {
		err := aiProvider.StreamChat(aiCtx, messages, responseStream)
		if err != nil {
			log.Printf("Error streaming chat: %v", err)
		}
		close(streamDone)
	}()

	// Process response
	for {
		select {
		case response, ok := <-responseStream:
			if !ok {
				log.Printf("Response stream closed")
				return nil
			}
			log.Printf("Received response: %s", response)

			// Create new context with timeout for each card update
			updateCtx, updateCancel := context.WithTimeout(ctx, 10*time.Second)
			
			// Update card content
			log.Printf("Updating card content for card ID: %s", cardID)
			_, err := handler.cardCreator.UpdateCardContent(updateCtx, cardID, response)
			
			// Clean up context
			updateCancel()
			
			if err != nil {
				log.Printf("Failed to update card content: %v", err)
				return err
			}
			log.Printf("Successfully updated card content")

		case err := <-streamDone:
			if err != nil {
				log.Printf("Stream ended with error: %v", err)
				return err
			}
			log.Printf("Stream ended successfully")
			return nil

		case <-aiCtx.Done():
			log.Printf("AI context cancelled: %v", aiCtx.Err())
			return aiCtx.Err()
		}
	}
}
