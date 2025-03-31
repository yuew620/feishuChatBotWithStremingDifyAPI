package initialization

import (
	"start-feishubot/services"
	"start-feishubot/services/cardcreator"
	"start-feishubot/services/core"
	"start-feishubot/services/feishu"
	"context"
)

var (
	sessionCache core.SessionCache
	cardCreator  core.CardCreator
	msgCache     core.MessageCache
)

// NewMessageCache creates a new message cache
func NewMessageCache() core.MessageCache {
	return core.NewMessageCache()
}

// NewSessionCache creates a new session cache
func NewSessionCache() core.SessionCache {
	return services.GetSessionCache()
}

// createCardAdapter adapts CardCreator.CreateCardEntity to cardpool.CreateCardFn
func createCardAdapter(creator core.CardCreator) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		// Use empty content for pool cards
		return creator.CreateCardEntity(ctx, "")
	}
}

// InitializeServices initializes all services
func InitializeServices() error {
	// Get config
	config := GetConfig()

	// Initialize Feishu config adapter
	feishuConfig := feishu.NewConfigAdapter(config)

	// Initialize card creator
	cardCreator = cardcreator.NewCardCreator(feishuConfig)

	// Initialize card pool with adapter
	if err := InitCardPool(createCardAdapter(cardCreator)); err != nil {
		return err
	}

	// Initialize session cache
	sessionCache = NewSessionCache()

	// Initialize message cache
	msgCache = NewMessageCache()

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
