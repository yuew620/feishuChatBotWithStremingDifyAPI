package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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

	// Stream chat
	go func() {
		err := aiProvider.StreamChat(aiCtx, messages, responseStream)
		if err != nil {
			fmt.Printf("Error streaming chat: %v\n", err)
		}
	}()

	// Process response
	for response := range responseStream {
		// Create new context with timeout for each card update
		cardCtx, cardCancel := context.WithTimeout(ctx, 10*time.Second)
		
		// Send message
		_, err := handler.cardCreator.CreateCardEntity(cardCtx, response)
		
		// Clean up context
		cardCancel()
		
		if err != nil {
			return err
		}
	}

	return nil
}
