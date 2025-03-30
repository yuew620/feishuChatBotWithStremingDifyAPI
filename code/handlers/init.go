package handlers

import (
	"context"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/services/factory"
)

var handler MessageHandlerInterface

// InitHandlers initializes the handlers
func InitHandlers() error {
	// Get service factory instance
	serviceFactory := factory.GetInstance()

	// Create message handler
	h := NewMessageHandler(
		serviceFactory.GetSessionCache(),
		serviceFactory.GetCardCreator(),
		serviceFactory.GetMsgCache(),
		serviceFactory.GetAIProvider(),
	)
	handler = h
	return nil
}

func Handler(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	return handler.msgReceivedHandler(ctx, event)
}

func ReadHandler(ctx context.Context, event *larkim.P2MessageReadV1) error {
	return nil
}

// Shutdown 关闭所有服务
func Shutdown() {
	// Nothing to do here since services are managed by factory
}
