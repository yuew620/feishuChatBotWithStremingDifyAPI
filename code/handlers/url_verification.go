package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
)

// UrlVerification represents the URL verification request from Feishu
type UrlVerification struct {
	Challenge string `json:"challenge"`
	Token     string `json:"token"`
	Type      string `json:"type"`
}

// HandleUrlVerification handles the URL verification request from Feishu
func HandleUrlVerification(c *gin.Context) bool {
	if c.Request.Header.Get("X-Lark-Request-Type") == "URL_VERIFICATION" {
		log.Printf("Received URL verification request")
		
		// Read raw body immediately to minimize latency
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("Failed to read request body: %v", err)
			return true
		}
		
		// Log the raw request for debugging
		log.Printf("Raw verification request: %s", string(body))
		
		var event UrlVerification
		if err := json.Unmarshal(body, &event); err != nil {
			log.Printf("Failed to parse URL verification request: %v", err)
			return true
		}

		// Verify the request type is correct
		if event.Type != "url_verification" {
			log.Printf("Invalid verification type: %s", event.Type)
			return true
		}
		
		// Return exactly what Feishu expects: {"challenge": "value"}
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, fmt.Sprintf(`{"challenge":"%s"}`, event.Challenge))
		log.Printf("Responded with challenge: %s", event.Challenge)
		return true
	}
	return false
}
