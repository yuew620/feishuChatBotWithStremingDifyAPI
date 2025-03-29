package initialization

import (
	"fmt"
	"start-feishubot/services/ai"
	"start-feishubot/services/ai/dify"
	difyservice "start-feishubot/services/dify"
	"sync"
	"time"
)

// AIConfigWrapper 配置适配器
type AIConfigWrapper struct {
	config *Config
}

func NewAIConfigWrapper(config *Config) *AIConfigWrapper {
	return &AIConfigWrapper{config: config}
}

// 实现ai.Config接口
func (w *AIConfigWrapper) GetProviderType() string {
	return w.config.AIProviderType
}

func (w *AIConfigWrapper) GetApiUrl() string {
	return w.config.AIApiUrl
}

func (w *AIConfigWrapper) GetApiKey() string {
	return w.config.AIApiKey
}

func (w *AIConfigWrapper) GetModel() string {
	return w.config.AIModel
}

func (w *AIConfigWrapper) GetTimeout() time.Duration {
	return time.Duration(w.config.AITimeout) * time.Second
}

func (w *AIConfigWrapper) GetMaxRetries() int {
	return w.config.AIMaxRetries
}

// 实现dify.Config接口
func (w *AIConfigWrapper) GetDifyApiUrl() string {
	return w.config.AIApiUrl
}

func (w *AIConfigWrapper) GetDifyApiKey() string {
	return w.config.AIApiKey
}

// InitAIProvider 初始化AI提供商
func InitAIProvider() (ai.Provider, error) {
	config := GetConfig()
	
	// 验证基本配置
	if err := validateAIConfig(config); err != nil {
		return nil, fmt.Errorf("invalid AI configuration: %v", err)
	}

	// 创建配置适配器
	aiConfig := NewAIConfigWrapper(config)

	// 获取工厂管理器
	factoryManager := ai.GetFactoryManager()

	// 注册支持的AI提供商工厂
	if err := registerFactories(factoryManager); err != nil {
		return nil, fmt.Errorf("failed to register factories: %v", err)
	}

	// 创建提供商实例
	provider, err := factoryManager.CreateProvider(aiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI provider: %v", err)
	}

	return provider, nil
}

// validateAIConfig 验证AI配置
func validateAIConfig(config *Config) error {
	if config.AIProviderType == "" {
		return fmt.Errorf("AI_PROVIDER_TYPE is required")
	}
	if config.AIApiUrl == "" {
		return fmt.Errorf("AI_API_URL is required")
	}
	if config.AIApiKey == "" {
		return fmt.Errorf("AI_API_KEY is required")
	}
	if config.AITimeout <= 0 {
		return fmt.Errorf("AI_TIMEOUT must be positive")
	}
	if config.AIMaxRetries < 0 {
		return fmt.Errorf("AI_MAX_RETRIES cannot be negative")
	}
	return nil
}

// registerFactories 注册所有支持的AI提供商工厂
func registerFactories(manager *ai.FactoryManager) error {
	// 注册Dify工厂
	if err := manager.RegisterFactory(ai.ProviderTypeDify, &dify.DifyFactory{}); err != nil {
		return fmt.Errorf("failed to register Dify factory: %v", err)
	}

	// 这里可以注册其他提供商的工厂
	// if err := manager.RegisterFactory(ai.ProviderTypeOpenAI, &openai.OpenAIFactory{}); err != nil {
	//     return fmt.Errorf("failed to register OpenAI factory: %v", err)
	// }

	return nil
}

// 全局Dify客户端实例
var difyClient *difyservice.DifyClient
var difyClientOnce sync.Once

// GetDifyClient 获取或创建Dify客户端实例
func GetDifyClient() *difyservice.DifyClient {
	difyClientOnce.Do(func() {
		config := GetConfig()
		difyClient = difyservice.NewDifyClient(NewAIConfigWrapper(config))
	})
	return difyClient
}

// ShutdownAIProvider 关闭AI提供商
func ShutdownAIProvider() error {
	factoryManager := ai.GetFactoryManager()
	return factoryManager.CloseAllProviders()
}
