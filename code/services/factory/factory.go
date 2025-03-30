package factory

import (
	"sync"
	"start-feishubot/services"
	"start-feishubot/services/cardcreator"
	"start-feishubot/services/dify"
)

var (
	sessionCache services.SessionServiceCacheInterface
	cardCreator  cardcreator.CardCreator
	msgCache     services.MessageCacheInterface
	difyClient   *dify.DifyClient
	
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

// GetDifyClient returns the Dify client instance
func GetDifyClient() *dify.DifyClient {
	serviceOnce.Do(initServices)
	return difyClient
}

// initServices initializes all services
func initServices() {
	sessionCache = services.GetSessionCache()
	cardCreator = cardcreator.NewCardCreator()
	msgCache = services.NewMessageCache()
	difyClient = dify.NewDifyClient()
}
