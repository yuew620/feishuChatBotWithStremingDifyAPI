package config

// Config defines the interface for configuration
type Config interface {
	// Feishu configuration
	GetFeishuAppID() string
	GetFeishuAppSecret() string
	GetFeishuAppVerificationToken() string

	// Dify configuration
	GetDifyAPIEndpoint() string
	GetDifyAPIKey() string

	// HTTP configuration
	GetHttpPort() string

	// General configuration
	IsInitialized() bool
}

// ConfigImpl implements the Config interface
type ConfigImpl struct {
	FeishuAppID                 string `json:"feishu_app_id"`
	FeishuAppSecret            string `json:"feishu_app_secret"`
	FeishuAppVerificationToken string `json:"feishu_app_verification_token"`
	DifyAPIEndpoint            string `json:"dify_api_endpoint"`
	DifyAPIKey                 string `json:"dify_api_key"`
	HttpPort                   string `json:"http_port"`
	Initialized               bool   `json:"-"`
}

func (c *ConfigImpl) GetFeishuAppID() string {
	return c.FeishuAppID
}

func (c *ConfigImpl) GetFeishuAppSecret() string {
	return c.FeishuAppSecret
}

func (c *ConfigImpl) GetFeishuAppVerificationToken() string {
	return c.FeishuAppVerificationToken
}

func (c *ConfigImpl) GetDifyAPIEndpoint() string {
	return c.DifyAPIEndpoint
}

func (c *ConfigImpl) GetDifyAPIKey() string {
	return c.DifyAPIKey
}

func (c *ConfigImpl) GetHttpPort() string {
	return c.HttpPort
}

func (c *ConfigImpl) IsInitialized() bool {
	return c.Initialized
}
