package factory

import (
	"sync"
	"start-feishubot/services/cardcreator"
	"start-feishubot/services/core"
)

// ServiceFactory manages service instances
type ServiceFactory struct {
	sessionCache core.SessionCache
	cardCreator  core.CardCreator
	msgCache     core.MessageCache
	aiProvider   core.AIProvider
}

var (
	instance *ServiceFactory
	once     sync.Once
)

// GetInstance returns the singleton instance of ServiceFactory
func GetInstance() *ServiceFactory {
	once.Do(func() {
		instance = &ServiceFactory{
			msgCache: core.NewMessageCache(),
		}
	})
	return instance
}

// SetSessionCache sets the session cache instance
func (f *ServiceFactory) SetSessionCache(cache core.SessionCache) {
	f.sessionCache = cache
}

// SetCardCreator sets the card creator instance
func (f *ServiceFactory) SetCardCreator(creator core.CardCreator) {
	f.cardCreator = creator
}

// SetAIProvider sets the AI provider instance
func (f *ServiceFactory) SetAIProvider(provider core.AIProvider) {
	f.aiProvider = provider
}

// GetSessionCache returns the session cache instance
func (f *ServiceFactory) GetSessionCache() core.SessionCache {
	return f.sessionCache
}

// GetCardCreator returns the card creator instance
func (f *ServiceFactory) GetCardCreator() core.CardCreator {
	return f.cardCreator
}

// GetMsgCache returns the message cache instance
func (f *ServiceFactory) GetMsgCache() core.MessageCache {
	return f.msgCache
}

// GetAIProvider returns the AI provider instance
func (f *ServiceFactory) GetAIProvider() core.AIProvider {
	return f.aiProvider
}
