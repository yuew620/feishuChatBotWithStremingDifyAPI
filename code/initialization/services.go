package initialization

import (
	"sync"
	"start-feishubot/services"
	"start-feishubot/services/cardcreator"
	"start-feishubot/services/dify"
	"start-feishubot/services/factory"
	"start-feishubot/services/feishu"
)

var (
	sessionCache services.SessionServiceCacheInterface
	cardCreator  *cardcreator.CardCreator
	msgCache     factory.MessageCache
	difyClient   *dify.DifyClient
	
	serviceOnce sync.Once
)

// GetSessionCache returns the session cache instance
func GetSessionCache() services.SessionServiceCacheInterface {
	serviceOnce.Do(initServices)
	return sessionCache
}

// GetCardCreator returns the card creator instance
func GetCardCreator() *cardcreator.CardCreator {
	serviceOnce.Do(initServices)
	return cardCreator
}

// GetMsgCache returns the message cache instance
func GetMsgCache() factory.MessageCache {
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
	msgCache = factory.NewMessageCache()
	
	config := GetConfig()
	feishuConfig := feishu.NewConfigAdapter(config)
	cardCreator = cardcreator.NewCardCreator(feishuConfig)
	
	difyConfig := dify.NewConfigAdapter(config)
	difyClient = dify.NewDifyClient(difyConfig)
}
