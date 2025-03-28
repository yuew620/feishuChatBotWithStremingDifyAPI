package ai

import (
	"context"
	"fmt"
	"time"
)

// Message 统一的消息结构
type Message struct {
	Role     string                 `json:"role"`
	Content  string                 `json:"content"`
	Metadata map[string]string      `json:"metadata,omitempty"`
}

// ValidateMessage 验证消息格式
func (m *Message) Validate() error {
	if m.Role == "" {
		return fmt.Errorf("message role cannot be empty")
	}
	if m.Content == "" {
		return fmt.Errorf("message content cannot be empty")
	}
	
	// 验证角色类型
	validRoles := map[string]bool{
		"system":    true,
		"user":      true,
		"assistant": true,
	}
	if !validRoles[m.Role] {
		return fmt.Errorf("invalid message role: %s", m.Role)
	}
	
	return nil
}

// Provider AI服务提供商接口
type Provider interface {
	// StreamChat 流式对话
	StreamChat(ctx context.Context, messages []Message, responseStream chan string) error
	// Close 关闭提供商，清理资源
	Close() error
}

// Config AI服务配置接口
type Config interface {
	GetProviderType() string
	GetApiUrl() string
	GetApiKey() string
	GetModel() string
	GetTimeout() time.Duration
	GetMaxRetries() int
}

// Factory 创建AI服务提供商的工厂
type Factory interface {
	CreateProvider(config Config) (Provider, error)
}

// Error 定义错误类型
type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// ErrorCode 错误码
type ErrorCode int

const (
	ErrInvalidConfig ErrorCode = iota + 1
	ErrProviderNotFound
	ErrConnectionFailed
	ErrTimeout
	ErrInvalidResponse
	ErrRateLimited
	ErrInvalidMessage
)

// NewError 创建新的错误
func NewError(code ErrorCode, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// IsTemporary 判断错误是否为临时错误
func (e *Error) IsTemporary() bool {
	switch e.Code {
	case ErrConnectionFailed, ErrTimeout, ErrRateLimited:
		return true
	default:
		return false
	}
}
