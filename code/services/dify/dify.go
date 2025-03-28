package dify

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"start-feishubot/initialization"
	"strings"
)

type DifyClient struct {
	config *initialization.Config
}

type Messages struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StreamRequest struct {
	Query    string     `json:"query"`
	Messages []Messages `json:"messages"`
	Stream   bool      `json:"stream"`
}

type StreamResponse struct {
	Event string `json:"event"`
	Data  struct {
		Text string `json:"text"`
	} `json:"data"`
}

func NewDifyClient(config *initialization.Config) *DifyClient {
	return &DifyClient{
		config: config,
	}
}

func (d *DifyClient) StreamChat(ctx context.Context, messages []Messages, responseStream chan string) error {
	// 构建请求体
	lastMsg := messages[len(messages)-1]
	historicalMessages := messages[:len(messages)-1]
	
	reqBody := StreamRequest{
		Query:    lastMsg.Content,
		Messages: historicalMessages,
		Stream:   true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("error marshaling request: %v", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", 
		fmt.Sprintf("%s/v1/chat-messages", d.config.DifyApiUrl), 
		strings.NewReader(string(jsonBody)))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.config.DifyApiKey))

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// 处理流式响应
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading stream: %v", err)
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		// 解析SSE数据
		data := strings.TrimPrefix(line, "data: ")
		var streamResp StreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			return fmt.Errorf("error unmarshaling response: %v", err)
		}

		// 处理不同的事件类型
		switch streamResp.Event {
		case "message":
			responseStream <- streamResp.Data.Text
		case "error":
			return fmt.Errorf("stream error: %s", streamResp.Data.Text)
		case "done":
			return nil
		}
	}
}
