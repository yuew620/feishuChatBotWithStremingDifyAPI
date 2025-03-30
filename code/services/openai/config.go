package openai

// Config represents OpenAI-specific configuration
type Config struct {
	OpenaiApiKeys []string
	OpenaiApiUrl  string
	OpenaiModel   string
	HttpProxy     string
	
	// Azure specific config
	AzureOn            bool
	AzureApiVersion    string
	AzureDeploymentName string
	AzureResourceName   string
	AzureOpenaiToken    string
	
	// HTTP client config
	OpenAIHttpClientTimeOut int
}

var globalConfig *Config

// InitConfig initializes the OpenAI configuration
func InitConfig(cfg *Config) {
	globalConfig = cfg
}

// GetConfig returns the OpenAI configuration
func GetConfig() *Config {
	return globalConfig
}
