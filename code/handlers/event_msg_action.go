package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/services/ai"
	"start-feishubot/services/core"
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

func processMessage(ctx context.Context, event *larkim.P2MessageReceiveV1, handler *MessageHandler) error {
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
	content, err := event.Event.Message.Content.MarshalJSON()
	if err != nil {
		return err
	}

	// Parse content
	var msg struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &msg); err != nil {
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

	// Get AI provider
	aiProvider := handler.dify

	// Stream chat
	go func() {
		err := aiProvider.StreamChat(ctx, messages, responseStream)
		if err != nil {
			fmt.Printf("Error streaming chat: %v\n", err)
		}
	}()

	// Process response
	for response := range responseStream {
		// Send message
		_, err = handler.cardCreator.CreateCardEntity(ctx, response)
		if err != nil {
			return err
		}
	}

	return nil
}
