package handlers

import (
	"encoding/json"
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
		
		// Read raw body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("Failed to read request body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request"})
			return true
		}
		
		// Log the raw request for debugging
		log.Printf("Raw verification request: %s", string(body))
		
		var event UrlVerification
		if err := json.Unmarshal(body, &event); err != nil {
			log.Printf("Failed to parse URL verification request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return true
		}
		
		// Return the exact same format as received
		response := map[string]interface{}{
			"challenge": event.Challenge,
		}
		
		log.Printf("Responding to URL verification with: %+v", response)
		c.JSON(http.StatusOK, response)
		return true
	}
	return false
}
