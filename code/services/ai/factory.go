package ai

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Common errors
var (
	ErrInvalidConfig = NewError("invalid configuration")
)

// Factory manages AI providers
type Factory struct {
	mu       sync.RWMutex
	config   Config
	provider Provider
}

// Config defines the configuration for AI providers
type Config struct {
	Provider     string `json:"provider"`
	APIEndpoint  string `json:"api_endpoint"`
	APIKey       string `json:"api_key"`
	MaxTokens    int    `json:"max_tokens"`
	Temperature  float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	StopWords   []string `json:"stop_words"`
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Provider == "" {
		return ErrInvalidConfig
	}
	if c.APIEndpoint == "" {
		return ErrInvalidConfig
	}
	if c.APIKey == "" {
		return ErrInvalidConfig
	}
	if c.MaxTokens <= 0 {
		return ErrInvalidConfig
	}
	if c.Temperature < 0 || c.Temperature > 1 {
		return ErrInvalidConfig
	}
	return nil
}

var (
	factory *Factory
	once    sync.Once
)

// GetFactory returns the singleton factory instance
func GetFactory() *Factory {
	once.Do(func() {
		factory = &Factory{}
	})
	return factory
}

// Initialize initializes the factory with configuration
func (f *Factory) Initialize(config Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	f.config = config
	return nil
}

// GetProvider returns the configured AI provider
func (f *Factory) GetProvider() (Provider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.provider != nil {
		return f.provider, nil
	}

	return nil, errors.New("provider not initialized")
}

// StreamChat streams chat messages using the configured provider
func (f *Factory) StreamChat(ctx context.Context, messages []Message, responseStream chan string) error {
	provider, err := f.GetProvider()
	if err != nil {
		return err
	}

	return provider.StreamChat(ctx, messages, responseStream)
}

// Close closes the factory and its provider
func (f *Factory) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.provider != nil {
		if err := f.provider.Close(); err != nil {
			return err
		}
		f.provider = nil
	}

	return nil
}
