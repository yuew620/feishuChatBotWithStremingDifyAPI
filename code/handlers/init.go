package handlers

import (
	"start-feishubot/initialization"
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
	cardPool := initialization.GetCardPool()

	// Create message handler
	messageHandler = &MessageHandler{
		sessionCache: sessionCache,
		cardCreator: cardCreator,
		msgCache:    msgCache,
		dify:        aiProvider,
		cardPool:    cardPool,
	}

	return nil
}

// Shutdown performs cleanup
func Shutdown() {
	// Add cleanup code here
}
