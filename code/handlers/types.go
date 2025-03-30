package handlers

import (
	"context"
	"errors"
	"time"

	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/services/ai"
	"start-feishubot/services/dify"
	"start-feishubot/services"
)

type SessionMode = services.SessionMode

// SessionStats contains session statistics
type SessionStats struct {
	TotalSessions      int32     `json:"total_sessions"`
	TotalMemoryUsedMB  float64   `json:"total_memory_used_mb"`
	ActiveUsers        int       `json:"active_users"`
	AvgSessionSize     float64   `json:"avg_session_size"`
	LastCleanupTime    time.Time `json:"last_cleanup_time"`
	CleanedSessions    int       `json:"cleaned_sessions"`
}

type SessionMeta = services.SessionMeta

// Forward declarations for external types
type CardCreator interface {
	CreateCard(content string) (string, error)
}

type MessageCacheInterface interface {
	Set(key string, value interface{})
	Get(key string) (interface{}, bool)
}

type SessionServiceCacheInterface interface {
	GetMessages(sessionId string) []ai.Message
	SetMessages(sessionId string, userId string, messages []ai.Message, cardId string, messageId string, conversationID string, cacheAddress string) error
	GetMode(sessionId string) SessionMode
	SetMode(sessionId string, mode SessionMode)
	Clear(sessionId string)
	ClearUserSessions(userId string)
	GetUserSessions(userId string) []string
	CleanExpiredSessions() int
	GetStats() SessionStats
	SetPicResolution(sessionId string, resolution string)
	GetPicResolution(sessionId string) string
	SetMsg(sessionId string, msg []ai.Message)
	GetSessionMeta(sessionId string) (*SessionMeta, bool)
	IsDuplicateMessage(userId string, messageId string) bool
	GetCardID(sessionId string, userId string, messageId string) (string, error)
	GetSessionInfo(userId string, messageId string) (*services.SessionMeta, error)
}

// HandlerType defines the type of handler
type HandlerType string

// Handler types
const (
	GroupHandler   HandlerType = "group"
	PrivateHandler HandlerType = "private"
	OtherHandler   HandlerType = "other"
)

// CardKind defines the type of card
type CardKind string

// Card kinds
const (
	ClearCardKind      CardKind = "clear"
	PicModeChangeKind  CardKind = "pic_mode_change"
	PicResolutionKind  CardKind = "pic_resolution"
	PicTextMoreKind    CardKind = "pic_text_more"
	PicVarMoreKind     CardKind = "pic_var_more"
	RoleTagsChooseKind CardKind = "role_tags_choose"
	RoleChooseKind     CardKind = "role_choose"
)

// CardChatType defines the type of chat
type CardChatType string

// Chat types
const (
	GroupChatType CardChatType = "group"
	UserChatType  CardChatType = "personal"
)

// CardMsg represents a card message
type CardMsg struct {
	Kind      CardKind
	ChatType  CardChatType
	SessionId string
	MsgId     string
	Value     interface{}
}

// CardHandlerFunc defines the function type for handling card actions
type CardHandlerFunc func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error)

// CardHandlerMeta defines the function type for creating card handlers
type CardHandlerMeta func(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc

// CardInfo contains information about a card
type CardInfo struct {
	CardEntityId string
	MessageId    string
	ElementId    string
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

// MessageHandler defines the message handler struct
type MessageHandler struct {
	sessionCache SessionServiceCacheInterface
	cardCreator  CardCreator
	msgCache     MessageCacheInterface
	dify        *dify.DifyClient
}

// MessageHandlerInterface defines the interface for message handlers
type MessageHandlerInterface interface {
	msgReceivedHandler(ctx context.Context, event *larkim.P2MessageReceiveV1) error
	cardHandler(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error)
	judgeIfMentionMe(mention []*larkim.MentionEvent) bool
}

// UserHandler implements MessageHandlerInterface
type UserHandler struct {
	MessageHandlerInterface
}

var (
	ErrNextHandler = errors.New("next handler")
)

// judgeChatType determines the type of chat
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
