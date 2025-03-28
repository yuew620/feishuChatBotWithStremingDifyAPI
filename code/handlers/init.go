package handlers

import (
	"context"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/initialization"
)

var handler MessageHandlerInterface

func InitHandlers(config initialization.Config) error {
	h, err := NewMessageHandler(config)
	if err != nil {
		return err
	}
	handler = h
	return nil
}

func Handler(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	return handler.msgReceivedHandler(ctx, event)
}

func ReadHandler(ctx context.Context, event *larkim.P2MessageReadV1) error {
	return nil
}
