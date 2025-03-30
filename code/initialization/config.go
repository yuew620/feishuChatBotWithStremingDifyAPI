package initialization

import (
	"fmt"
	"github.com/spf13/pflag"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"start-feishubot/config"
)

// Config extends the base config with initialization-specific fields
type Config struct {
	config.Config
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
	HttpProxy                          string
	AzureOn                            bool
	AzureApiVersion                    string
	AzureDeploymentName                string
	AzureResourceName                  string
	AzureOpenaiToken                   string
	AccessControlMaxCountPerUserPerDay int
	OpenAIHttpClientTimeOut            int
}

var (
	cfg    = pflag.StringP("config", "c", "./config.yaml", "apiserver config file path.")
	config *Config
	once   sync.Once
)

/*
GetConfig will call LoadConfig once and return a global singleton, you should always use this function to get config
*/
func GetConfig() *Config {

	once.Do(func() {
		config = LoadConfig(*cfg)
		config.Initialized = true
	})

	return config
}

/*
LoadConfig will load config and should only be called once, you should always use GetConfig to get config rather than
call this function directly
*/
func LoadConfig(cfg string) *Config {
	viper.SetConfigFile(cfg)
	viper.ReadInConfig()
	viper.AutomaticEnv()
	//content, err := ioutil.ReadFile("config.yaml")
	//if err != nil {
	//	fmt.Println("Error reading file:", err)
	//}
	//fmt.Println(string(content))

	cfg := &Config{
		Config: config.Config{
			AppID:              getViperStringValue("APP_ID", ""),
			AppSecret:         getViperStringValue("APP_SECRET", ""),
			VerificationToken: getViperStringValue("APP_VERIFICATION_TOKEN", ""),
			EncryptKey:        getViperStringValue("APP_ENCRYPT_KEY", ""),
			OpenaiApiKeys:     getViperStringArray("OPENAI_KEY", nil),
			OpenaiModel:       getViperStringValue("OPENAI_MODEL", "gpt-3.5-turbo"),
			HttpPort:          getViperStringValue("HTTP_PORT", "9000"),
			FeishuAppID:      getViperStringValue("APP_ID", ""),
			FeishuAppSecret:  getViperStringValue("APP_SECRET", ""),
			AccessControlEnable: getViperBoolValue("ACCESS_CONTROL_ENABLE", false),
			AccessControlUsers:  strings.Split(getViperStringValue("ACCESS_CONTROL_USERS", ""), ","),
		},
		EnableLog:                          getViperBoolValue("ENABLE_LOG", false),
		FeishuBotName:                      getViperStringValue("BOT_NAME", ""),
		HttpsPort:                          getViperIntValue("HTTPS_PORT", 9001),
		UseHttps:                           getViperBoolValue("USE_HTTPS", false),
		CertFile:                           getViperStringValue("CERT_FILE", "cert.pem"),
		KeyFile:                            getViperStringValue("KEY_FILE", "key.pem"),
		HttpProxy:                          getViperStringValue("HTTP_PROXY", ""),
		AzureOn:                            getViperBoolValue("AZURE_ON", false),
		AzureApiVersion:                    getViperStringValue("AZURE_API_VERSION", "2023-03-15-preview"),
		AzureDeploymentName:                getViperStringValue("AZURE_DEPLOYMENT_NAME", ""),
		AzureResourceName:                  getViperStringValue("AZURE_RESOURCE_NAME", ""),
		AzureOpenaiToken:                   getViperStringValue("AZURE_OPENAI_TOKEN", ""),
		AccessControlMaxCountPerUserPerDay: getViperIntValue("ACCESS_CONTROL_MAX_COUNT_PER_USER_PER_DAY", 0),
		OpenAIHttpClientTimeOut:            getViperIntValue("OPENAI_HTTP_CLIENT_TIMEOUT", 550),
		
		// AI Provider配置
		AIProviderType:                     getViperStringValue("AI_PROVIDER_TYPE", "dify"),
		AIApiUrl:                           getViperStringValue("AI_API_URL", ""),
		AIApiKey:                           getViperStringValue("AI_API_KEY", ""),
		AIModel:                            getViperStringValue("AI_MODEL", ""),
		AITimeout:                          getViperIntValue("AI_TIMEOUT", 30),
		AIMaxRetries:                       getViperIntValue("AI_MAX_RETRIES", 3),
	}

	return config
}

func getViperStringValue(key string, defaultValue string) string {
	value := viper.GetString(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// OPENAI_KEY: sk-xxx,sk-xxx,sk-xxx
// result:[sk-xxx sk-xxx sk-xxx]
func getViperStringArray(key string, defaultValue []string) []string {
	value := viper.GetString(key)
	if value == "" {
		return defaultValue
	}
	raw := strings.Split(value, ",")
	return filterFormatKey(raw)
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

func (config *Config) GetCertFile() string {
	if config.CertFile == "" {
		return "cert.pem"
	}
	if _, err := os.Stat(config.CertFile); err != nil {
		fmt.Printf("Certificate file %s does not exist, using default file cert.pem\n", config.CertFile)
		return "cert.pem"
	}
	return config.CertFile
}

func (config *Config) GetKeyFile() string {
	if config.KeyFile == "" {
		return "key.pem"
	}
	if _, err := os.Stat(config.KeyFile); err != nil {
		fmt.Printf("Key file %s does not exist, using default file key.pem\n", config.KeyFile)
		return "key.pem"
	}
	return config.KeyFile
}

// 过滤出 "sk-" 开头的 key
func filterFormatKey(keys []string) []string {
	var result []string
	for _, key := range keys {
		if strings.HasPrefix(key, "sk-") {
			result = append(result, key)
		}
	}
	return result

}
