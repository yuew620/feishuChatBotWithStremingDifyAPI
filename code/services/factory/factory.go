package factory

import (
	"sync"
	"start-feishubot/initialization"
	"start-feishubot/services"
	"start-feishubot/services/dify"
)

// CardCreator interface for creating cards
type CardCreator interface {
	CreateCard(content string) (string, error)
}

// MessageCache interface for message caching
type MessageCache interface {
	Set(key string, value interface{})
	Get(key string) (interface{}, bool)
}

// cardCreatorImpl implements CardCreator
type cardCreatorImpl struct{}

func (c *cardCreatorImpl) CreateCard(content string) (string, error) {
	return content, nil
}

// messageCacheImpl implements MessageCache
type messageCacheImpl struct {
	cache sync.Map
}

func (m *messageCacheImpl) Set(key string, value interface{}) {
	m.cache.Store(key, value)
}

func (m *messageCacheImpl) Get(key string) (interface{}, bool) {
	return m.cache.Load(key)
}

func NewCardCreator() CardCreator {
	return &cardCreatorImpl{}
}

func NewMessageCache() MessageCache {
	return &messageCacheImpl{}
}

var (
	sessionCache services.SessionServiceCacheInterface
	cardCreator  CardCreator
	msgCache     MessageCache
	difyClient   *dify.DifyClient
	
	serviceOnce sync.Once
)

// GetSessionCache returns the session cache instance
func GetSessionCache() services.SessionServiceCacheInterface {
	serviceOnce.Do(initServices)
	return sessionCache
}

// GetCardCreator returns the card creator instance
func GetCardCreator() CardCreator {
	serviceOnce.Do(initServices)
	return cardCreator
}

// GetMsgCache returns the message cache instance
func GetMsgCache() MessageCache {
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
	cardCreator = NewCardCreator()
	msgCache = NewMessageCache()
	
	config := initialization.GetConfig()
	difyConfig := dify.NewConfigAdapter(config)
	difyClient = dify.NewDifyClient(difyConfig)
}
