package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"
	"log"
	"net/http"
	"start-feishubot/handlers"
	"start-feishubot/initialization"
	"time"
)

func main() {
	// Parse command line flags
	pflag.Parse()

	// Load configuration
	config := initialization.GetConfig()
	if !config.IsInitialized() {
		log.Fatal("Failed to load configuration")
	}

	// Set global config for handlers
	handlers.SetConfig(config)

	// Initialize all services
	log.Printf("[Main] ===== Starting service initialization =====")
	startTime := time.Now()
	if err := initialization.InitializeServices(); err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}
	log.Printf("[Main] ===== Service initialization completed in %v =====", time.Since(startTime))

	// Initialize handlers
	if err := handlers.InitHandlers(); err != nil {
		log.Fatalf("Failed to initialize handlers: %v", err)
	}

	// Register shutdown hook
	defer func() {
		handlers.Shutdown()
	}()

	// Set up Gin
	r := gin.Default()

	// Register routes
	r.POST("/webhook/event", func(c *gin.Context) {
		if err := handlers.Handler(c); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	})

	r.POST("/webhook/card", func(c *gin.Context) {
		if err := handlers.Handler(c); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	})

	// Start server
	addr := fmt.Sprintf(":%s", config.GetHttpPort())
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
