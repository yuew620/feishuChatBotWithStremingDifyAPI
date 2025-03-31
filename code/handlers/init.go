package handlers

import (
	"start-feishubot/initialization"
	"start-feishubot/services/core"
)

var (
	messageHandler *MessageHandler
)

// InitHandlers initializes all handlers
func InitHandlers() error {
	// Get services
	sessionCache := initialization.GetSessionCache()
	cardCreator := initialization.GetCardCreator()
	msgCache := initialization.GetMsgCache()
	aiProvider := initialization.GetAIProvider()

	// Create message handler
	messageHandler = NewMessageHandler(
		sessionCache,
		cardCreator,
		msgCache,
		aiProvider,
	)

	return nil
}

// Shutdown performs cleanup
func Shutdown() {
	// Add cleanup code here
}
