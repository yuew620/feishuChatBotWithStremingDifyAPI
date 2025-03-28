package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"start-feishubot/initialization"
)

// UrlVerification represents the URL verification request from Feishu
type UrlVerification struct {
	Challenge string `json:"challenge"`
	Token     string `json:"token"`
	Type      string `json:"type"`
}

// HandleUrlVerification handles the URL verification request from Feishu
func HandleUrlVerification(c *gin.Context) bool {
	// Read raw body immediately to minimize latency
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		return false
	}
	
	// Restore body for later use
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	
	// Try to parse as verification request
	var event UrlVerification
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Not a verification request (parse error): %v", err)
		return false
	}

	// Check if this is a verification request
	if event.Type != "url_verification" {
		log.Printf("Not a verification request (type=%s)", event.Type)
		return false
	}

	log.Printf("Handling URL verification request")

	// Only verify token if it's configured
	config := initialization.GetConfig()
	if config.FeishuAppVerificationToken != "" {
		if event.Token != config.FeishuAppVerificationToken {
			log.Printf("Invalid verification token: %s", event.Token)
			return false
		}
		log.Printf("Verification token matched")
	} else {
		log.Printf("Verification token check skipped (not configured)")
	}
	
	// Return exactly what Feishu expects: {"challenge": "value"}
	c.Header("Content-Type", "application/json")
	response := fmt.Sprintf(`{"challenge":"%s"}`+"\n", event.Challenge)
	c.String(http.StatusOK, response)
	log.Printf("Responded with challenge: %s", event.Challenge)
	return true
}
