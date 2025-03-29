package handlers

import (
	"context"
	"fmt"
	"log"
	"start-feishubot/initialization"
	
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// CreateCardEntity 创建飞书卡片实体
func CreateCardEntity(ctx context.Context, content string) (string, error) {
	client := initialization.GetLarkClient()
	if client == nil {
		return "", fmt.Errorf("lark client not initialized")
	}

	// 如果没有提供内容，使用默认内容
	if content == "" {
		content = "正在处理中..."
	}

	// 创建卡片
	card, err := client.Im.Message.Create(ctx, &larkim.CreateMessageReqBuilder{
		ReceiveId: "",
		Content:   content,
		MsgType:   "interactive",
	})

	if err != nil {
		log.Printf("Failed to create card: %v", err)
		return "", fmt.Errorf("failed to create card: %w", err)
	}

	// 返回卡片ID
	return card.Data.MessageId, nil
}
