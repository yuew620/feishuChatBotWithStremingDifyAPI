package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"
	"log"
	"start-feishubot/handlers"
	"start-feishubot/initialization"
)

func main() {
	// Parse command line flags
	pflag.Parse()

	// Load configuration
	config := initialization.GetConfig()
	if !config.Initialized {
		log.Fatal("Failed to load configuration")
	}

	// Initialize all services
	if err := initialization.InitializeServices(); err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}

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
	r.POST("/webhook/event", handlers.Handler)
	r.POST("/webhook/card", handlers.Handler)

	// Start server
	addr := fmt.Sprintf(":%s", config.HttpPort)
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
