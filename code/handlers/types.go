package handlers

import (
	"context"
	"start-feishubot/services"

	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// HandlerType 定义处理器类型
type HandlerType string

const (
	GroupHandler   HandlerType = "group"
	PrivateHandler HandlerType = "private"
	OtherHandler   HandlerType = "other"
)

// MessageHandlerInterface 消息处理器接口
type MessageHandlerInterface interface {
	// 处理消息接收
	msgReceivedHandler(ctx context.Context, event *larkim.P2MessageReceiveV1) error
	// 处理卡片动作
	cardHandler(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error)
	// 判断是否提到我
	judgeIfMentionMe(mention []*larkim.MentionEvent) bool
}

// Resolution 图片分辨率类型
type Resolution string

const (
	Resolution256  Resolution = "256x256"
	Resolution512  Resolution = "512x512"
	Resolution1024 Resolution = "1024x1024"
)

// 扩展 SessionServiceCacheInterface 接口
type ExtendedSessionServiceCacheInterface interface {
	services.SessionServiceCacheInterface
	SetPicResolution(sessionId string, resolution Resolution)
	GetPicResolution(sessionId string) Resolution
}
