package feishu

import "start-feishubot/services/config"

// ConfigAdapter adapts initialization.Config to config.FeishuConfig
type ConfigAdapter struct {
	config config.FeishuConfig
}

func NewConfigAdapter(config config.FeishuConfig) *ConfigAdapter {
	return &ConfigAdapter{config: config}
}

func (a *ConfigAdapter) GetFeishuAppID() string {
	return a.config.GetFeishuAppID()
}

func (a *ConfigAdapter) GetFeishuAppSecret() string {
	return a.config.GetFeishuAppSecret()
}
