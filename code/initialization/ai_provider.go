package initialization

import (
	"context"
	"errors"
	"fmt"
	"log"
	"start-feishubot/services/ai"
	"start-feishubot/services/dify"
	"sync"
)

var (
	aiProvider ai.Provider
	aiOnce     sync.Once
)

// InitAIProvider initializes the AI provider
func InitAIProvider() (ai.Provider, error) {
	var initErr error
	aiOnce.Do(func() {
		config := GetConfig()
		if !config.IsInitialized() {
			initErr = errors.New("configuration not initialized")
			return
		}

		// Create Dify client
		difyConfig := dify.NewConfigAdapter(config)
		difyClient := dify.NewDifyClient(difyConfig)

		// Set as global provider
		aiProvider = difyClient
	})

	if initErr != nil {
		return nil, fmt.Errorf("failed to initialize AI provider: %v", initErr)
	}

	if aiProvider == nil {
		return nil, errors.New("AI provider not initialized")
	}

	return aiProvider, nil
}

// GetAIProvider returns the initialized AI provider
func GetAIProvider() ai.Provider {
	provider, err := InitAIProvider()
	if err != nil {
		log.Printf("Failed to get AI provider: %v", err)
		return nil
	}
	return provider
}

// ShutdownAIProvider gracefully shuts down the AI provider
func ShutdownAIProvider() error {
	if aiProvider == nil {
		return nil
	}

	// Add any cleanup logic here if needed
	aiProvider = nil
	return nil
}

// StreamChat implements ai.Provider interface for testing
func StreamChat(ctx context.Context, messages []ai.Message, responseStream chan string) error {
	provider := GetAIProvider()
	if provider == nil {
		return errors.New("AI provider not available")
	}
	return provider.StreamChat(ctx, messages, responseStream)
}
