package initialization

import (
	"context"
	"start-feishubot/services"
	"start-feishubot/services/cardcreator"
	"start-feishubot/services/cardservice"
	"start-feishubot/services/dify"
	"start-feishubot/services/factory"
	"start-feishubot/services/feishu"
)

// InitializeServices initializes all services
func InitializeServices() error {
	// Get service factory instance
	serviceFactory := factory.GetInstance()

	// Initialize session cache
	sessionCache := services.GetSessionCache()
	serviceFactory.SetSessionCache(sessionCache)

	// Get configuration
	config := GetConfig()

	// Initialize Feishu services
	feishuConfig := feishu.NewConfigAdapter(config)
	cardCreator := cardcreator.NewCardCreator(feishuConfig)
	serviceFactory.SetCardCreator(cardCreator)

	// Initialize card pool
	cardservice.InitCardPool(func(ctx context.Context) (string, error) {
		return cardCreator.CreateCardEntity(ctx, "")
	})

	// Initialize Dify services
	difyConfig := dify.NewConfigAdapter(config)
	difyClient := dify.NewDifyClient(difyConfig)
	serviceFactory.SetAIProvider(difyClient)

	// Initialize AI provider
	_, err := InitAIProvider()
	if err != nil {
		return err
	}

	return nil
}

// ShutdownServices gracefully shuts down all services
func ShutdownServices() error {
	// Shutdown card pool
	cardservice.ShutdownCardPool()

	// Shutdown AI provider
	if err := ShutdownAIProvider(); err != nil {
		return err
	}

	return nil
}
