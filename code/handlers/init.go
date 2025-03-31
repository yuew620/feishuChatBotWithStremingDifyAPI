package handlers

import (
	"log"
	"start-feishubot/initialization"
	"time"
)

var (
	messageHandler *MessageHandler
)

// InitHandlers initializes all handlers
func InitHandlers() error {
	log.Printf("[Handlers] ===== Starting handlers initialization =====")
	startTime := time.Now()

	// Get services
	log.Printf("[Handlers] Getting required services")
	sessionCache := initialization.GetSessionCache()
	cardCreator := initialization.GetCardCreator()
	msgCache := initialization.GetMsgCache()
	aiProvider := initialization.GetAIProvider()
	cardPool := initialization.GetCardPool()
	log.Printf("[Handlers] All required services retrieved")

	// Create message handler
	log.Printf("[Handlers] Creating message handler")
	messageHandler = &MessageHandler{
		sessionCache: sessionCache,
		cardCreator: cardCreator,
		msgCache:    msgCache,
		dify:        aiProvider,
		cardPool:    cardPool,
	}
	log.Printf("[Handlers] Message handler created")

	log.Printf("[Handlers] ===== Handlers initialization completed in %v =====", time.Since(startTime))
	return nil
}

// Shutdown performs cleanup
func Shutdown() {
	// Add cleanup code here
}
