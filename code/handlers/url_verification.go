package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"start-feishubot/services/config"
)

// Challenge represents the challenge request
type Challenge struct {
	Challenge string `json:"challenge"`
	Token     string `json:"token"`
	Type      string `json:"type"`
}

// VerifyURL verifies the URL with challenge
func VerifyURL(body []byte, cfg config.Config) (interface{}, error) {
	// Parse challenge
	var challenge Challenge
	if err := json.Unmarshal(body, &challenge); err != nil {
		return nil, err
	}

	// Verify token
	token := cfg.GetFeishuAppVerificationToken()
	if challenge.Token != token {
		return nil, fmt.Errorf("invalid token")
	}

	// Return challenge response
	return map[string]string{
		"challenge": challenge.Challenge,
	}, nil
}

// VerifyRequest verifies the request signature
func VerifyRequest(r *http.Request, body []byte, cfg config.Config) error {
	// Get signature
	signature := r.Header.Get("X-Lark-Signature")
	if signature == "" {
		return fmt.Errorf("missing signature")
	}

	// Calculate expected signature
	token := cfg.GetFeishuAppVerificationToken()
	timestamp := r.Header.Get("X-Lark-Request-Timestamp")
	nonce := r.Header.Get("X-Lark-Request-Nonce")
	expected := sha256.Sum256([]byte(fmt.Sprintf("%s%s%s%s", timestamp, nonce, token, string(body))))

	// Verify signature
	if fmt.Sprintf("%x", expected) != signature {
		return fmt.Errorf("invalid signature")
	}

	return nil
}
