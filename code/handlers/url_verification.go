package handlers

import (
	"github.com/gin-gonic/gin"
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
		var event UrlVerification
		if err := c.ShouldBindJSON(&event); err != nil {
			log.Printf("Failed to parse URL verification request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return true
		}
		log.Printf("Responding to URL verification with challenge: %s", event.Challenge)
		c.JSON(http.StatusOK, gin.H{"challenge": event.Challenge})
		return true
	}
	return false
}
