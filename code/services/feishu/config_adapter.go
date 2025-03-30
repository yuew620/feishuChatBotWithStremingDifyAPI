package feishu

import "start-feishubot/initialization"

// ConfigAdapter adapts initialization.Config to feishu.Config
type ConfigAdapter struct {
	config *initialization.Config
}

func NewConfigAdapter(config *initialization.Config) *ConfigAdapter {
	return &ConfigAdapter{config: config}
}

func (a *ConfigAdapter) GetFeishuAppID() string {
	return a.config.FeishuAppID
}

func (a *ConfigAdapter) GetFeishuAppSecret() string {
	return a.config.FeishuAppSecret
}
