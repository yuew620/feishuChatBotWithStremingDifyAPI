package dify

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"start-feishubot/services/ai"
)

// DifyClient implements core.AIProvider interface
type DifyClient struct {
	config *ConfigAdapter
}

// NewDifyClient creates a new Dify client
func NewDifyClient(config *ConfigAdapter) *DifyClient {
	return &DifyClient{
		config: config,
	}
}

// StreamChat implements core.AIProvider interface
func (d *DifyClient) StreamChat(ctx context.Context, messages []ai.Message, responseStream chan string) error {
	// Convert messages to Dify format
	difyMessages := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		difyMessages[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	// Prepare request body
	requestBody := map[string]interface{}{
		"messages": difyMessages,
		"stream":   true,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", d.config.GetAPIEndpoint()+"/chat/completions", strings.NewReader(string(jsonBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.config.GetAPIKey())

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// Read response stream
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read response: %v", err)
		}

		// Skip empty lines
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse SSE data
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		// Parse JSON
		var response struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &response); err != nil {
			return fmt.Errorf("failed to parse response: %v", err)
		}

		// Send content to stream
		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case responseStream <- response.Choices[0].Delta.Content:
			}
		}
	}

	return nil
}
