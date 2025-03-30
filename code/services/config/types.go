package config

// Config defines the interface for configuration
type Config interface {
	// Feishu configuration
	GetFeishuAppID() string
	GetFeishuAppSecret() string

	// Dify configuration
	GetDifyAPIEndpoint() string
	GetDifyAPIKey() string

	// HTTP configuration
	GetHttpPort() string

	// General configuration
	IsInitialized() bool
}
