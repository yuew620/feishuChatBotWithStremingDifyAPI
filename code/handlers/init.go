package handlers

import (
	"context"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/initialization"
	"start-feishubot/services/accesscontrol"
	"start-feishubot/services/cardservice"
	"start-feishubot/services/factory"
)

var handler MessageHandlerInterface

func InitHandlers(cfg *initialization.Config) error {
	// 初始化AI提供商
	_, err := initialization.InitAIProvider()
	if err != nil {
		return err
	}

	// 初始化访问控制
	if cfg.AccessControlEnable {
		err := accesscontrol.InitAccessControl(&cfg.Config)
		if err != nil {
			return err
		}
	}

	// 初始化卡片池
	cardservice.InitCardPool(func(ctx context.Context) (string, error) {
		return CreateCardEntity(ctx, "")
	})

	// 创建消息处理器
	sessionCache := factory.GetSessionCache()
	cardCreator := factory.GetCardCreator()
	msgCache := factory.GetMsgCache()
	difyClient := initialization.GetDifyClient()
	
	h := NewMessageHandler(sessionCache, cardCreator, msgCache, difyClient)
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
