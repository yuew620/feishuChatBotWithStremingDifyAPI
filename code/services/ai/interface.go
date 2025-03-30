package ai

import (
	"context"
)

// Message represents a chat message
type Message struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

// Validate validates the message
func (m *Message) Validate() error {
	if m.Role == "" {
		return ErrEmptyRole
	}
	if m.Content == "" {
		return ErrEmptyContent
	}
	return nil
}

// Provider defines the interface for AI providers
type Provider interface {
	// StreamChat streams chat messages
	StreamChat(ctx context.Context, messages []Message, responseStream chan string) error
	
	// Close closes the provider and cleans up resources
	Close() error
}

// Common errors
var (
	ErrEmptyRole    = NewError("empty role")
	ErrEmptyContent = NewError("empty content")
)

// Error represents an AI error
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// NewError creates a new AI error
func NewError(message string) error {
	return &Error{Message: message}
}
