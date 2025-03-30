package initialization

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"start-feishubot/services/config"
)

// ConfigImpl implements the Config interface
type ConfigImpl struct {
	FeishuAppID     string `json:"feishu_app_id"`
	FeishuAppSecret string `json:"feishu_app_secret"`
	DifyAPIEndpoint string `json:"dify_api_endpoint"`
	DifyAPIKey      string `json:"dify_api_key"`
	HttpPort        string `json:"http_port"`
	Initialized     bool   `json:"-"`
}

var globalConfig *ConfigImpl

// GetConfig returns the global configuration instance
func GetConfig() config.Config {
	if globalConfig == nil {
		globalConfig = &ConfigImpl{}
		if err := loadConfig(); err != nil {
			log.Printf("Failed to load config: %v", err)
			return globalConfig
		}
		globalConfig.Initialized = true
	}
	return globalConfig
}

// loadConfig loads configuration from file
func loadConfig() error {
	// Try to load from config.yaml first
	configPath := filepath.Join("config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		data, err := ioutil.ReadFile(configPath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, globalConfig); err != nil {
			return err
		}
		return nil
	}

	// If config.yaml doesn't exist, try environment variables
	globalConfig.FeishuAppID = os.Getenv("FEISHU_APP_ID")
	globalConfig.FeishuAppSecret = os.Getenv("FEISHU_APP_SECRET")
	globalConfig.DifyAPIEndpoint = os.Getenv("DIFY_API_ENDPOINT")
	globalConfig.DifyAPIKey = os.Getenv("DIFY_API_KEY")
	globalConfig.HttpPort = os.Getenv("HTTP_PORT")
	if globalConfig.HttpPort == "" {
		globalConfig.HttpPort = "8080"
	}

	return nil
}

// Implementation of Config interface

func (c *ConfigImpl) GetFeishuAppID() string {
	return c.FeishuAppID
}

func (c *ConfigImpl) GetFeishuAppSecret() string {
	return c.FeishuAppSecret
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
