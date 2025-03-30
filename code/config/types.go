package config

// Config represents the application configuration
type Config struct {
	AppID                string
	AppSecret           string
	VerificationToken   string
	EncryptKey          string
	HttpPort           string
	FeishuAppID        string
	FeishuAppSecret    string
}
