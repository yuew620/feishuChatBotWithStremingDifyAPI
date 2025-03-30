package initialization

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"start-feishubot/services/config"
)

var engine *gin.Engine

// InitGin initializes the Gin engine
func InitGin() (*gin.Engine, error) {
	if engine != nil {
		return engine, nil
	}

	// Get configuration
	cfg := GetConfig()
	if !cfg.IsInitialized() {
		return nil, fmt.Errorf("configuration not initialized")
	}

	// Create Gin engine
	engine = gin.New()

	// Add middleware
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())
	engine.Use(requestLogger())

	return engine, nil
}

// GetGin returns the initialized Gin engine
func GetGin() *gin.Engine {
	if engine == nil {
		engine, _ = InitGin()
	}
	return engine
}

// corsMiddleware adds CORS headers to responses
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// requestLogger logs incoming requests
func requestLogger() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
	})
}
