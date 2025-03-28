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
	Inputs          map[string]string `json:"inputs"`
	Query           string            `json:"query"`
	ResponseMode    string            `json:"response_mode"`
	ConversationId  string            `json:"conversation_id"`
	User            string            `json:"user"`
}

type StreamResponse struct {
	Event string `json:"event"`
	Data  struct {
		Text string `json:"text"`
	} `json:"data"`
}

// mustMarshal 将数据序列化为JSON，如果出错则panic
func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("json marshal error: %v", err))
	}
	return data
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
	
	// 构建消息历史
	var messageHistory []map[string]string
	for _, msg := range historicalMessages {
		messageHistory = append(messageHistory, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	reqBody := StreamRequest{
		Inputs: map[string]string{
			"history": string(mustMarshal(messageHistory)),
		},
		Query:           lastMsg.Content,
		ResponseMode:    "streaming",
		ConversationId:  "",
		User:            "feishu-bot",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("error marshaling request: %v", err)
	}

	// 构建完整的API URL（处理末尾斜杠）
	baseUrl := strings.TrimRight(d.config.DifyApiUrl, "/")
	apiUrl := fmt.Sprintf("%s/v1/chat-messages", baseUrl)

	// 打印请求详情
	fmt.Printf("Sending request to Dify:\nURL: %s\nHeaders: %v\nBody: %s\n", 
		apiUrl,
		map[string]string{
			"Content-Type": "application/json",
			"Authorization": fmt.Sprintf("Bearer %s", d.config.DifyApiKey),
		},
		string(jsonBody))

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", 
		apiUrl, 
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

	// 打印响应状态码和头部
	fmt.Printf("Dify response:\nStatus: %s\nHeaders: %v\n", 
		resp.Status, 
		resp.Header)

	// 如果状态码不是200，读取并打印错误响应
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

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
