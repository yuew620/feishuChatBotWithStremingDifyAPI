package initialization

import (
	"sync"
	"start-feishubot/services"
	"start-feishubot/services/cardcreator"
	"start-feishubot/services/openai"
)

var (
	sessionCache services.SessionServiceCacheInterface
	cardCreator  cardcreator.CardCreator
	msgCache     services.MessageCacheInterface
	openAIService *openai.ChatGPT
	
	serviceOnce sync.Once
)

// GetSessionCache returns the session cache instance
func GetSessionCache() services.SessionServiceCacheInterface {
	serviceOnce.Do(initServices)
	return sessionCache
}

// GetCardCreator returns the card creator instance
func GetCardCreator() cardcreator.CardCreator {
	serviceOnce.Do(initServices)
	return cardCreator
}

// GetMsgCache returns the message cache instance
func GetMsgCache() services.MessageCacheInterface {
	serviceOnce.Do(initServices)
	return msgCache
}

// GetOpenAIService returns the OpenAI service instance
func GetOpenAIService() *openai.ChatGPT {
	serviceOnce.Do(initServices)
	return openAIService
}

// initServices initializes all services
func initServices() {
	sessionCache = services.GetSessionCache()
	cardCreator = cardcreator.NewCardCreator()
	msgCache = services.NewMessageCache()
	openAIService = openai.NewChatGPT()
}
