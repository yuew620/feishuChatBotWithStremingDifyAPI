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

// 判断聊天类型
func judgeChatType(event *larkim.P2MessageReceiveV1) HandlerType {
	chatType := event.Event.Message.ChatType
	switch *chatType {
	case "group":
		return GroupHandler
	case "p2p":
		return PrivateHandler
	default:
		return OtherHandler
	}
}

// UserHandler 用户处理器类型
type UserHandler struct {
	MessageHandlerInterface
}
