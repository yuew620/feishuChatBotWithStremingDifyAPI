package handlers

import (
	"context"
	"start-feishubot/services/core"
)

// NewMessageHandler creates a new message handler
func NewMessageHandler(
	sessionCache core.SessionCache,
	cardCreator core.CardCreator,
	msgCache core.MessageCache,
	aiProvider core.AIProvider,
) *MessageHandler {
	return &MessageHandler{
		sessionCache: sessionCache,
		cardCreator: cardCreator,
		msgCache:    msgCache,
		dify:        aiProvider,
	}
}
