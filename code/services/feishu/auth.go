package feishu

import (
	"start-feishubot/services/config"
)

// Config represents Feishu configuration
type Config struct {
	AppID     string
	AppSecret string
}

// NewConfig creates a new Feishu config from global config
func NewConfig(cfg config.Config) *Config {
	return &Config{
		AppID:     cfg.GetFeishuAppID(),
		AppSecret: cfg.GetFeishuAppSecret(),
	}
}
