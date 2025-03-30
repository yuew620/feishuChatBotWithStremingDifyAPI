package cardcreator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"start-feishubot/services/feishu"
	"time"
)

// Config defines the interface for card creator configuration
type Config interface {
	feishu.Config
}

// CardCreator handles card creation
type CardCreator struct {
	config Config
}

// NewCardCreator creates a new CardCreator instance
func NewCardCreator(config Config) *CardCreator {
	return &CardCreator{
		config: config,
	}
}

// CreateCardEntity 创建飞书卡片实体
func (c *CardCreator) CreateCardEntity(ctx context.Context, content string) (string, error) {
	startTime := time.Now()
	log.Printf("[Timing] Starting card entity creation")
	
	// 获取tenant_access_token
	tokenStart := time.Now()
	token, err := feishu.GetTenantAccessToken(ctx, c.config)
	if err != nil {
		return "", fmt.Errorf("failed to get tenant_access_token: %w", err)
	}
	
	// 构建卡片JSON 2.0结构，严格按照飞书官方文档
	cardJSON := map[string]interface{}{
		"schema": "2.0",
		"config": map[string]interface{}{
			"streaming_mode": true,
			"summary": map[string]interface{}{
				"content": "[生成中]",
			},
			"streaming_config": map[string]interface{}{
				"print_frequency_ms": map[string]interface{}{
					"default": 30,
					"android": 25,
					"ios": 40,
					"pc": 50,
				},
				"print_step": map[string]interface{}{
					"default": 2,
					"android": 3,
					"ios": 4,
					"pc": 5,
				},
				"print_strategy": "fast",
			},
		},
		"body": map[string]interface{}{
			"elements": []map[string]interface{}{
				{
					"tag": "markdown",
					"content": content,
					"element_id": "content_block",
				},
			},
		},
	}
	
	// 序列化卡片JSON
	cardJSONStr, err := json.Marshal(cardJSON)
	if err != nil {
		return "", fmt.Errorf("failed to marshal card JSON: %w", err)
	}
	
	// 构建请求体
	reqBody := map[string]interface{}{
		"type": "card_json",
		"data": string(cardJSONStr),
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// 构建请求URL
	url := "https://open.feishu.cn/open-apis/cardkit/v1/cards/"
	log.Printf("Creating card entity with URL: %s", url)
	
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// 发送请求
	requestStart := time.Now()
	log.Printf("[Timing] Token fetch took: %v ms", time.Since(tokenStart).Milliseconds())
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	log.Printf("[Timing] Card entity API request took: %v ms", time.Since(requestStart).Milliseconds())
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			CardID string `json:"card_id"`
		} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	if result.Code != 0 {
		return "", fmt.Errorf("API error: code=%d, msg=%s", result.Code, result.Msg)
	}
	
	log.Printf("Successfully created card entity with ID: %s", result.Data.CardID)
	log.Printf("[Timing] Total card entity creation took: %v ms", time.Since(startTime).Milliseconds())
	return result.Data.CardID, nil
}
