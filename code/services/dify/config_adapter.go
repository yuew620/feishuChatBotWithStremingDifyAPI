package dify

import "start-feishubot/initialization"

// ConfigAdapter adapts initialization.Config to dify.Config
type ConfigAdapter struct {
	config *initialization.Config
}

func NewConfigAdapter(config *initialization.Config) *ConfigAdapter {
	return &ConfigAdapter{config: config}
}

func (a *ConfigAdapter) GetDifyApiUrl() string {
	return a.config.AIApiUrl
}

func (a *ConfigAdapter) GetDifyApiKey() string {
	return a.config.AIApiKey
}
