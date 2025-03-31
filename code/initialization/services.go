package initialization

import (
	"start-feishubot/services/core"
)

var (
	sessionCache core.SessionCache
	cardCreator  core.CardCreator
	msgCache     core.MessageCache
	aiProvider   core.AIProvider
)

// InitializeServices initializes all services
func InitializeServices() error {
	// Initialize services here
	return nil
}

// GetSessionCache returns the session cache service
func GetSessionCache() core.SessionCache {
	return sessionCache
}

// GetCardCreator returns the card creator service
func GetCardCreator() core.CardCreator {
	return cardCreator
}

// GetMsgCache returns the message cache service
func GetMsgCache() core.MessageCache {
	return msgCache
}

// GetAIProvider returns the AI provider service
func GetAIProvider() core.AIProvider {
	return aiProvider
}
