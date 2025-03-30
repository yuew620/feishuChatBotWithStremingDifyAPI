package dify

import (
	"start-feishubot/services/config"
)

// Config represents Dify configuration
type Config struct {
	APIEndpoint string
	APIKey      string
}

// NewConfig creates a new Dify config from global config
func NewConfig(cfg config.Config) *Config {
	return &Config{
		APIEndpoint: cfg.GetDifyAPIEndpoint(),
		APIKey:      cfg.GetDifyAPIKey(),
	}
}
