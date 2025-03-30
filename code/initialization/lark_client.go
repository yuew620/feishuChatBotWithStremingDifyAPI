package initialization

import (
	"fmt"
	lark "github.com/larksuite/oapi-sdk-go/v3"
)

var client *lark.Client

// InitLarkClient initializes the Lark client
func InitLarkClient() (*lark.Client, error) {
	if client != nil {
		return client, nil
	}

	// Get configuration
	cfg := GetConfig()
	if !cfg.IsInitialized() {
		return nil, fmt.Errorf("configuration not initialized")
	}

	// Create Lark client
	client = lark.NewClient(
		cfg.GetFeishuAppID(),
		cfg.GetFeishuAppSecret(),
	)

	return client, nil
}

// GetLarkClient returns the initialized Lark client
func GetLarkClient() *lark.Client {
	if client == nil {
		client, _ = InitLarkClient()
	}
	return client
}
