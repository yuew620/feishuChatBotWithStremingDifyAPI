package config

// FeishuConfig defines the interface for Feishu configuration
type FeishuConfig interface {
	GetFeishuAppID() string
	GetFeishuAppSecret() string
}

// DifyConfig defines the interface for Dify configuration
type DifyConfig interface {
	GetDifyApiUrl() string
	GetDifyApiKey() string
}
