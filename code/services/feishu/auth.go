package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"start-feishubot/services/config"
)

// GetTenantAccessToken 获取tenant_access_token
func GetTenantAccessToken(ctx context.Context, config config.FeishuConfig) (string, error) {
	// 构建请求体
	reqBody := map[string]string{
		"app_id":     config.GetFeishuAppID(),
		"app_secret": config.GetFeishuAppSecret(),
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Code              int    `json:"code"`
		Msg              string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	if result.Code != 0 {
		return "", fmt.Errorf("API error: code=%d, msg=%s", result.Code, result.Msg)
	}

	return result.TenantAccessToken, nil
}
