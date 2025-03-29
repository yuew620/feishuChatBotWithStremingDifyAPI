package handlers

import (
	"context"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/initialization"
	"start-feishubot/services/accesscontrol"
	"start-feishubot/services/ai"
	"start-feishubot/services/cardcreator"
	"start-feishubot/services/cardservice"
)

var handler MessageHandlerInterface

func InitHandlers(config initialization.Config) error {
	// 初始化AI提供商
	provider, err := ai.InitAIProvider()
	if err != nil {
		return err
	}

	// 初始化访问控制
	if config.AccessControlEnable {
		accesscontrol.InitAccessControl()
	}

	// 初始化卡片池
	cardservice.InitCardPool(func(ctx context.Context, content string) (string, error) {
		return CreateCardEntity(ctx, content)
	})

	// 创建消息处理器
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

// Shutdown 关闭所有服务
func Shutdown() {
	// 关闭卡片池
	cardservice.ShutdownCardPool()
}
