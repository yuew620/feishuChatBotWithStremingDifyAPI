package dify

// Config defines the interface for Dify configuration
type Config interface {
	// GetDifyApiUrl returns the Dify API URL
	GetDifyApiUrl() string
	
	// GetDifyApiKey returns the Dify API key
	GetDifyApiKey() string
}
