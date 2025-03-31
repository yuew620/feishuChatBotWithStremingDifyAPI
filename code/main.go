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

	log.Printf("[Main] ===== Starting application initialization =====")
	mainStartTime := time.Now()

	// Load configuration
	log.Printf("[Main] Loading configuration...")
	config := initialization.GetConfig()
	if !config.IsInitialized() {
		log.Fatal("[Main] Failed to load configuration")
	}
	log.Printf("[Main] Configuration loaded successfully")

	// Set global config for handlers
	log.Printf("[Main] Setting global config for handlers")
	handlers.SetConfig(config)

	// Initialize all services
	log.Printf("[Main] Starting service initialization")
	serviceStartTime := time.Now()
	if err := initialization.InitializeServices(); err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}
	log.Printf("[Main] Service initialization completed in %v", time.Since(serviceStartTime))

	// Initialize handlers
	log.Printf("[Main] Starting handlers initialization")
	handlersStartTime := time.Now()
	if err := handlers.InitHandlers(); err != nil {
		log.Fatalf("[Main] Failed to initialize handlers: %v", err)
	}
	log.Printf("[Main] Handlers initialization completed in %v", time.Since(handlersStartTime))

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

	log.Printf("[Main] ===== Application initialization completed in %v =====", time.Since(mainStartTime))

	// Start server
	addr := fmt.Sprintf(":%s", config.GetHttpPort())
	log.Printf("[Main] Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
