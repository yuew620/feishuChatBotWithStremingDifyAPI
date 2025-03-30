package feishu

import (
	"start-feishubot/services/config"
	lark "github.com/larksuite/oapi-sdk-go/v3"
)

// ConfigAdapter adapts the config interface for Feishu services
type ConfigAdapter struct {
	config config.Config
	client *lark.Client
}

// NewConfigAdapter creates a new config adapter
func NewConfigAdapter(config config.Config) *ConfigAdapter {
	client := lark.NewClient(
		config.GetFeishuAppID(),
		config.GetFeishuAppSecret(),
	)
	return &ConfigAdapter{
		config: config,
		client: client,
	}
}

// GetLarkClient returns the Feishu client instance
func (c *ConfigAdapter) GetLarkClient() *lark.Client {
	return c.client
}

// GetFeishuAppID returns the Feishu app ID
func (c *ConfigAdapter) GetFeishuAppID() string {
	return c.config.GetFeishuAppID()
}

// GetFeishuAppSecret returns the Feishu app secret
func (c *ConfigAdapter) GetFeishuAppSecret() string {
	return c.config.GetFeishuAppSecret()
}
