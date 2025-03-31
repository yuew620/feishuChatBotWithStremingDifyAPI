package initialization

import (
	"context"
	"fmt"
	"log"
	"start-feishubot/services"
	"start-feishubot/services/cardcreator"
	"start-feishubot/services/cardpool"
	"start-feishubot/services/core"
	"start-feishubot/services/feishu"
)

var (
	sessionCache core.SessionCache
	cardCreator  core.CardCreator
	msgCache     core.MessageCache
	cardPool     *cardpool.CardPool
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
	log.Printf("[Services] ===== Starting services initialization =====")
	startTime := time.Now()

	// Get config
	config := GetConfig()
	log.Printf("[Services] Config loaded")

	// Initialize Feishu config adapter
	feishuConfig := feishu.NewConfigAdapter(config)
	log.Printf("[Services] Feishu config adapter initialized")

	// Initialize card creator
	cardCreator = cardcreator.NewCardCreator(feishuConfig)
	log.Printf("[Services] Card creator initialized")

	// Initialize card pool with adapter
	log.Printf("[Services] Starting card pool initialization")
	if err := InitCardPool(createCardAdapter(cardCreator)); err != nil {
		return fmt.Errorf("failed to initialize card pool: %w", err)
	}
	cardPool = GetCardPool()
	log.Printf("[Services] Card pool initialized with size: %d", cardPool.GetPoolSize())

	// Initialize session cache
	sessionCache = NewSessionCache()
	log.Printf("[Services] Session cache initialized")

	// Initialize message cache
	msgCache = NewMessageCache()
	log.Printf("[Services] Message cache initialized")

	log.Printf("[Services] ===== Services initialization completed in %v =====", time.Since(startTime))

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

// GetCardPool returns the card pool service
func GetCardPool() *cardpool.CardPool {
	return cardPool
}

// ShutdownServices performs cleanup of all services
func ShutdownServices() {
	if cardPool != nil {
		cardPool.Stop()
	}
}
