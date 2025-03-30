package config

// Config represents the application configuration
type Config struct {
	AppID                string
	AppSecret           string
	VerificationToken   string
	EncryptKey          string
	OpenaiApiKeys       []string
	OpenaiModel         string
	HttpPort           string
	FeishuAppID        string
	FeishuAppSecret    string
	AccessControlEnable bool
	AccessControlUsers  []string
}
