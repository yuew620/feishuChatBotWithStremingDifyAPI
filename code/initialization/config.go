package initialization

import (
	"fmt"
	"github.com/spf13/pflag"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/viper"
	baseconfig "start-feishubot/config"
	"start-feishubot/services/config"
)

// Config extends the base config with initialization-specific fields
type Config struct {
	baseconfig.Config
	// Additional initialization-specific fields
	Initialized                        bool
	EnableLog                          bool
	AIProviderType                     string
	AIApiUrl                           string
	AIApiKey                           string
	AIModel                            string
	AITimeout                          int
	AIMaxRetries                       int
	FeishuBotName                      string
	HttpsPort                          int
	UseHttps                           bool
	CertFile                           string
	KeyFile                            string
}

// Implement config.FeishuConfig
func (c *Config) GetFeishuAppID() string {
	return c.FeishuAppID
}

func (c *Config) GetFeishuAppSecret() string {
	return c.FeishuAppSecret
}

// Implement config.DifyConfig
func (c *Config) GetDifyApiUrl() string {
	return c.AIApiUrl
}

func (c *Config) GetDifyApiKey() string {
	return c.AIApiKey
}

var (
	cfgPath = pflag.StringP("config", "c", "./config.yaml", "apiserver config file path.")
	configInstance *Config
	once   sync.Once
)

/*
GetConfig will call LoadConfig once and return a global singleton, you should always use this function to get config
*/
func GetConfig() *Config {
	once.Do(func() {
		configInstance = LoadConfig(*cfgPath)
		configInstance.Initialized = true
	})

	return configInstance
}

/*
LoadConfig will load config and should only be called once, you should always use GetConfig to get config rather than
call this function directly
*/
func LoadConfig(cfgPath string) *Config {
	viper.SetConfigFile(cfgPath)
	viper.ReadInConfig()
	viper.AutomaticEnv()
	//content, err := ioutil.ReadFile("config.yaml")
	//if err != nil {
	//	fmt.Println("Error reading file:", err)
	//}
	//fmt.Println(string(content))

	configObj := &Config{
		Config: baseconfig.Config{
			AppID:              getViperStringValue("APP_ID", ""),
			AppSecret:         getViperStringValue("APP_SECRET", ""),
			VerificationToken: getViperStringValue("APP_VERIFICATION_TOKEN", ""),
			EncryptKey:        getViperStringValue("APP_ENCRYPT_KEY", ""),
			HttpPort:          getViperStringValue("HTTP_PORT", "9000"),
			FeishuAppID:      getViperStringValue("APP_ID", ""),
			FeishuAppSecret:  getViperStringValue("APP_SECRET", ""),
		},
		EnableLog:                          getViperBoolValue("ENABLE_LOG", false),
		FeishuBotName:                      getViperStringValue("BOT_NAME", ""),
		HttpsPort:                          getViperIntValue("HTTPS_PORT", 9001),
		UseHttps:                           getViperBoolValue("USE_HTTPS", false),
		CertFile:                           getViperStringValue("CERT_FILE", "cert.pem"),
		KeyFile:                            getViperStringValue("KEY_FILE", "key.pem"),
		
		// AI Provider配置
		AIProviderType:                     getViperStringValue("AI_PROVIDER_TYPE", "dify"),
		AIApiUrl:                           getViperStringValue("AI_API_URL", ""),
		AIApiKey:                           getViperStringValue("AI_API_KEY", ""),
		AIModel:                            getViperStringValue("AI_MODEL", "gpt-3.5-turbo"),
		AITimeout:                          getViperIntValue("AI_TIMEOUT", 30),
		AIMaxRetries:                       getViperIntValue("AI_MAX_RETRIES", 3),
	}

	return configObj
}

func getViperStringValue(key string, defaultValue string) string {
	value := viper.GetString(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getViperIntValue(key string, defaultValue int) int {
	value := viper.GetString(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		fmt.Printf("Invalid value for %s, using default value %d\n", key, defaultValue)
		return defaultValue
	}
	return intValue
}

func getViperBoolValue(key string, defaultValue bool) bool {
	value := viper.GetString(key)
	if value == "" {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		fmt.Printf("Invalid value for %s, using default value %v\n", key, defaultValue)
		return defaultValue
	}
	return boolValue
}

func (c *Config) GetCertFile() string {
	if c.CertFile == "" {
		return "cert.pem"
	}
	if _, err := os.Stat(c.CertFile); err != nil {
		fmt.Printf("Certificate file %s does not exist, using default file cert.pem\n", c.CertFile)
		return "cert.pem"
	}
	return c.CertFile
}

func (c *Config) GetKeyFile() string {
	if c.KeyFile == "" {
		return "key.pem"
	}
	if _, err := os.Stat(c.KeyFile); err != nil {
		fmt.Printf("Key file %s does not exist, using default file key.pem\n", c.KeyFile)
		return "key.pem"
	}
	return c.KeyFile
}
