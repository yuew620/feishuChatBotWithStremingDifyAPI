package initialization

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"start-feishubot/services/config"
	"time"
)

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

var globalConfig *ConfigImpl

// GetConfig returns the global configuration instance
func GetConfig() config.Config {
	if globalConfig == nil {
		log.Printf("[Config] ===== Starting configuration initialization =====")
		startTime := time.Now()
		
		globalConfig = &ConfigImpl{}
		if err := loadConfig(); err != nil {
			log.Printf("[Config] Failed to load config: %v", err)
			return globalConfig
		}
		globalConfig.Initialized = true
		
		log.Printf("[Config] ===== Configuration initialization completed in %v =====", time.Since(startTime))
	}
	return globalConfig
}

// loadConfig loads configuration from file
func loadConfig() error {
	// Try to load from config.yaml first
	configPath := filepath.Join("config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		log.Printf("[Config] Loading configuration from file: %s", configPath)
		data, err := ioutil.ReadFile(configPath)
		if err != nil {
			log.Printf("[Config] Failed to read config file: %v", err)
			return err
		}
		if err := json.Unmarshal(data, globalConfig); err != nil {
			log.Printf("[Config] Failed to parse config file: %v", err)
			return err
		}
		log.Printf("[Config] Successfully loaded configuration from file")
		return nil
	}

	// If config.yaml doesn't exist, try environment variables
	log.Printf("[Config] Config file not found, loading from environment variables")
	globalConfig.FeishuAppID = os.Getenv("FEISHU_APP_ID")
	globalConfig.FeishuAppSecret = os.Getenv("FEISHU_APP_SECRET")
	globalConfig.FeishuAppVerificationToken = os.Getenv("FEISHU_APP_VERIFICATION_TOKEN")
	globalConfig.DifyAPIEndpoint = os.Getenv("DIFY_API_ENDPOINT")
	globalConfig.DifyAPIKey = os.Getenv("DIFY_API_KEY")
	globalConfig.HttpPort = os.Getenv("HTTP_PORT")
	if globalConfig.HttpPort == "" {
		globalConfig.HttpPort = "8080"
		log.Printf("[Config] Using default HTTP port: %s", globalConfig.HttpPort)
	}
	log.Printf("[Config] Successfully loaded configuration from environment variables")

	return nil
}

// Implementation of Config interface

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
