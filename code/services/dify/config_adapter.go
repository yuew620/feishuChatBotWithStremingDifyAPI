package dify

import (
	"start-feishubot/services/config"
)

// ConfigAdapter adapts the config interface for Dify services
type ConfigAdapter struct {
	config config.Config
}

// NewConfigAdapter creates a new config adapter
func NewConfigAdapter(config config.Config) *ConfigAdapter {
	return &ConfigAdapter{
		config: config,
	}
}

// GetAPIEndpoint returns the Dify API endpoint
func (c *ConfigAdapter) GetAPIEndpoint() string {
	return c.config.GetDifyAPIEndpoint()
}

// GetAPIKey returns the Dify API key
func (c *ConfigAdapter) GetAPIKey() string {
	return c.config.GetDifyAPIKey()
}
