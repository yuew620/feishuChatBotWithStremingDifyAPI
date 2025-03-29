package handlers

import (
	"context"
	"errors"

	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// CardMsg represents a card message
type CardMsg struct {
	Kind      string
	SessionId string
	MsgId     string
	Value     interface{}
}

// CardHandlerFunc defines the function type for handling card actions
type CardHandlerFunc func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error)

// CardInfo contains information about a card
type CardInfo struct {
	CardId string
}

// MsgInfo contains information about a message
type MsgInfo struct {
	handlerType HandlerType
	msgType     string
	sessionId   *string
	msgId       *string
	chatId      string
	qParsed     string
	userId      string
	mention     []*larkim.MentionEvent
}

// ActionInfo contains information about an action
type ActionInfo struct {
	ctx     *context.Context
	info    *MsgInfo
	handler *MessageHandler
}

// Action defines the interface for actions
type Action interface {
	Execute(a *ActionInfo) bool
}

var (
	ErrNextHandler = errors.New("next handler")
)

// Card action kinds
const (
	PicResolutionKind  = "pic_resolution"
	PicModeChangeKind  = "pic_mode_change"
	PicTextMoreKind    = "pic_text_more"
	ClearCardKind      = "clear_card"
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
