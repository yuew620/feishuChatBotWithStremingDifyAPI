package ai

import (
	"fmt"
	"sync"
	"time"
)

// ProviderType 定义AI提供商类型
type ProviderType string

const (
	ProviderTypeDify    ProviderType = "dify"
	ProviderTypeOpenAI  ProviderType = "openai"
	ProviderTypeChatGPT ProviderType = "chatgpt"
)

// ProviderConfig AI提供商配置实现
type ProviderConfig struct {
	providerType ProviderType
	apiUrl       string
	apiKey       string
	model        string
	timeout      time.Duration
	maxRetries   int
}

func NewProviderConfig(providerType ProviderType, apiUrl, apiKey, model string) *ProviderConfig {
	return &ProviderConfig{
		providerType: providerType,
		apiUrl:       apiUrl,
		apiKey:       apiKey,
		model:        model,
		timeout:      30 * time.Second, // 默认30秒超时
		maxRetries:   3,               // 默认最多重试3次
	}
}

// WithTimeout 设置超时时间
func (c *ProviderConfig) WithTimeout(timeout time.Duration) *ProviderConfig {
	c.timeout = timeout
	return c
}

// WithMaxRetries 设置最大重试次数
func (c *ProviderConfig) WithMaxRetries(maxRetries int) *ProviderConfig {
	c.maxRetries = maxRetries
	return c
}

func (c *ProviderConfig) GetProviderType() string {
	return string(c.providerType)
}

func (c *ProviderConfig) GetApiUrl() string {
	return c.apiUrl
}

func (c *ProviderConfig) GetApiKey() string {
	return c.apiKey
}

func (c *ProviderConfig) GetModel() string {
	return c.model
}

func (c *ProviderConfig) GetTimeout() time.Duration {
	return c.timeout
}

func (c *ProviderConfig) GetMaxRetries() int {
	return c.maxRetries
}

// Validate 验证配置
func (c *ProviderConfig) Validate() error {
	if c.providerType == "" {
		return NewError(ErrInvalidConfig, "provider type cannot be empty", nil)
	}
	if c.apiUrl == "" {
		return NewError(ErrInvalidConfig, "API URL cannot be empty", nil)
	}
	if c.apiKey == "" {
		return NewError(ErrInvalidConfig, "API key cannot be empty", nil)
	}
	if c.timeout <= 0 {
		return NewError(ErrInvalidConfig, "timeout must be positive", nil)
	}
	if c.maxRetries < 0 {
		return NewError(ErrInvalidConfig, "max retries cannot be negative", nil)
	}
	return nil
}

// FactoryManager 工厂管理器
type FactoryManager struct {
	factories map[ProviderType]Factory
	providers map[string]Provider // 缓存已创建的Provider实例
	mu        sync.RWMutex
}

// 全局工厂管理器实例
var (
	factoryManager *FactoryManager
	once          sync.Once
)

// GetFactoryManager 获取工厂管理器单例
func GetFactoryManager() *FactoryManager {
	once.Do(func() {
		factoryManager = &FactoryManager{
			factories: make(map[ProviderType]Factory),
			providers: make(map[string]Provider),
		}
	})
	return factoryManager
}

// RegisterFactory 注册AI提供商工厂
func (m *FactoryManager) RegisterFactory(providerType ProviderType, factory Factory) error {
	if factory == nil {
		return NewError(ErrInvalidConfig, "factory cannot be nil", nil)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.factories[providerType]; exists {
		return NewError(ErrInvalidConfig, fmt.Sprintf("factory already registered for provider type: %s", providerType), nil)
	}

	m.factories[providerType] = factory
	return nil
}

// CreateProvider 创建AI提供商实例
func (m *FactoryManager) CreateProvider(config Config) (Provider, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查缓存中是否已存在Provider实例
	cacheKey := fmt.Sprintf("%s-%s", config.GetProviderType(), config.GetApiKey())
	if provider, exists := m.providers[cacheKey]; exists {
		return provider, nil
	}

	providerType := ProviderType(config.GetProviderType())
	factory, ok := m.factories[providerType]
	if !ok {
		return nil, NewError(ErrProviderNotFound, fmt.Sprintf("no factory registered for provider type: %s", providerType), nil)
	}

	provider, err := factory.CreateProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// 缓存Provider实例
	m.providers[cacheKey] = provider
	return provider, nil
}

// CloseAllProviders 关闭所有Provider实例
func (m *FactoryManager) CloseAllProviders() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for _, provider := range m.providers {
		if err := provider.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	m.providers = make(map[string]Provider)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing providers: %v", errs)
	}
	return nil
}

func validateConfig(config Config) error {
	if config == nil {
		return NewError(ErrInvalidConfig, "config cannot be nil", nil)
	}
	
	if config.GetProviderType() == "" {
		return NewError(ErrInvalidConfig, "provider type cannot be empty", nil)
	}
	
	if config.GetApiUrl() == "" {
		return NewError(ErrInvalidConfig, "API URL cannot be empty", nil)
	}
	
	if config.GetApiKey() == "" {
		return NewError(ErrInvalidConfig, "API key cannot be empty", nil)
	}
	
	if config.GetTimeout() <= 0 {
		return NewError(ErrInvalidConfig, "timeout must be positive", nil)
	}
	
	if config.GetMaxRetries() < 0 {
		return NewError(ErrInvalidConfig, "max retries cannot be negative", nil)
	}
	
	return nil
}
