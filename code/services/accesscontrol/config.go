package accesscontrol

import (
	"start-feishubot/config"
	"sync"
)

var (
	accessConfig *config.Config
	configOnce sync.Once
)

// InitConfig initializes the access control configuration
func InitConfig(cfg *config.Config) {
	configOnce.Do(func() {
		accessConfig = cfg
	})
}

// GetConfig returns the access control configuration
func GetConfig() *config.Config {
	return accessConfig
}
