package dify

import "start-feishubot/services/config"

// ConfigAdapter adapts initialization.Config to config.DifyConfig
type ConfigAdapter struct {
	config config.DifyConfig
}

func NewConfigAdapter(config config.DifyConfig) *ConfigAdapter {
	return &ConfigAdapter{config: config}
}

func (a *ConfigAdapter) GetDifyApiUrl() string {
	return a.config.GetDifyApiUrl()
}

func (a *ConfigAdapter) GetDifyApiKey() string {
	return a.config.GetDifyApiKey()
}
