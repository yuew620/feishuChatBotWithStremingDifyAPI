package core

import (
	"context"
	"sync"
	"time"
	"start-feishubot/services/ai"
)

// MessageCache interface for message caching
type MessageCache interface {
	Set(key string, value interface{})
	Get(key string) (interface{}, bool)
	IfProcessed(key string) bool
	TagProcessed(key string)
}

// CardCreator interface for creating cards
type CardCreator interface {
	CreateCardEntity(ctx context.Context, content string) (string, error)
}

// AIProvider interface for AI services
type AIProvider interface {
	StreamChat(ctx context.Context, messages []ai.Message, responseStream chan string) error
}

// SessionStats contains session statistics
type SessionStats struct {
	TotalSessions      int32     `json:"total_sessions"`
	TotalMemoryUsedMB  float64   `json:"total_memory_used_mb"`
	ActiveUsers        int       `json:"active_users"`
	AvgSessionSize     float64   `json:"avg_session_size"`
	LastCleanupTime    time.Time `json:"last_cleanup_time"`
	CleanedSessions    int       `json:"cleaned_sessions"`
}

// SessionMode defines the type of session
type SessionMode string

const (
	ModePicCreate SessionMode = "pic_create"
	ModePicVary   SessionMode = "pic_vary"
	ModeGPT       SessionMode = "gpt"
)

// SessionMeta contains session metadata
type SessionMeta struct {
	Mode           SessionMode  `json:"mode"`
	Messages       []ai.Message `json:"messages,omitempty"`
	UserId         string      `json:"user_id"`
	UpdatedAt      time.Time   `json:"updated_at"`
	MessageNum     int         `json:"message_num"`
	Size           int64       `json:"size"`
	PicResolution  string      `json:"pic_resolution,omitempty"`
	SystemMsg      []ai.Message `json:"system_msg,omitempty"`
	CardId         string      `json:"card_id,omitempty"`
	MessageId      string      `json:"message_id,omitempty"`
	ConversationID string      `json:"conversation_id,omitempty"`
	CacheAddress   string      `json:"cache_address,omitempty"`
}

// SessionCache interface for session management
type SessionCache interface {
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
	GetSessionInfo(userId string, messageId string) (*SessionMeta, error)
}

// Basic MessageCache implementation
type messageCacheImpl struct {
	cache sync.Map
	processed sync.Map
}

func (m *messageCacheImpl) Set(key string, value interface{}) {
	m.cache.Store(key, value)
}

func (m *messageCacheImpl) Get(key string) (interface{}, bool) {
	return m.cache.Load(key)
}

func (m *messageCacheImpl) IfProcessed(key string) bool {
	_, exists := m.processed.Load(key)
	return exists
}

func (m *messageCacheImpl) TagProcessed(key string) {
	m.processed.Store(key, true)
}

// NewMessageCache creates a new message cache instance
func NewMessageCache() MessageCache {
	return &messageCacheImpl{}
}
