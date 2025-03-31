package cardcreator

import (
	"context"
	"errors"
	"log"
	"time"
	"start-feishubot/services/feishu"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// CardCreator implements core.CardCreator interface
type CardCreator struct {
	config *feishu.ConfigAdapter
}

// NewCardCreator creates a new card creator instance
func NewCardCreator(config *feishu.ConfigAdapter) *CardCreator {
	return &CardCreator{
		config: config,
	}
}

// CreateCardEntity implements core.CardCreator interface
func (c *CardCreator) CreateCardEntity(ctx context.Context, content string) (string, error) {
	log.Printf("[CardCreator] Starting card entity creation at %v", time.Now().Format("15:04:05"))
	startTime := time.Now()

	// Use Feishu API to create card entity
	client := c.config.GetLarkClient()
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType("interactive").
			Content(content).
			Build()).
		Build()

	log.Printf("[CardCreator] Creating card entity with URL: https://open.feishu.cn/open-apis/cardkit/v1/cards/ at %v", time.Now().Format("15:04:05"))

	// Record token fetch time
	tokenTime := time.Since(startTime)
	log.Printf("[CardCreator] Token fetch took: %d ms at %v", tokenTime.Milliseconds(), time.Now().Format("15:04:05"))

	resp, err := client.Im.Message.Create(ctx, req)
	if err != nil {
		return "", err
	}

	// Record API request time
	apiTime := time.Since(startTime) - tokenTime
	log.Printf("[CardCreator] Card entity API request took: %d ms at %v", apiTime.Milliseconds(), time.Now().Format("15:04:05"))

	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", errors.New("failed to get message ID from response")
	}

	log.Printf("[CardCreator] Successfully created card entity with ID: %s at %v", *resp.Data.MessageId, time.Now().Format("15:04:05"))

	// Record total time
	totalTime := time.Since(startTime)
	log.Printf("[CardCreator] Total card entity creation took: %d ms at %v", totalTime.Milliseconds(), time.Now().Format("15:04:05"))

	return *resp.Data.MessageId, nil
}

// UpdateCardContent updates the content of an existing card
func (c *CardCreator) UpdateCardContent(ctx context.Context, cardID string, content string) (string, error) {
	log.Printf("[CardCreator] Starting card content update at %v", time.Now().Format("15:04:05"))
	startTime := time.Now()

	// Use Feishu API to update card content
	client := c.config.GetLarkClient()
	req := larkim.NewUpdateMessageReqBuilder().
		MessageId(cardID).
		Body(larkim.NewUpdateMessageReqBodyBuilder().
			Content(content).
			Build()).
		Build()

	log.Printf("[CardCreator] Updating card content with URL: https://open.feishu.cn/open-apis/cardkit/v1/cards/%s at %v", cardID, time.Now().Format("15:04:05"))

	// Record token fetch time
	tokenTime := time.Since(startTime)
	log.Printf("[CardCreator] Token fetch took: %d ms at %v", tokenTime.Milliseconds(), time.Now().Format("15:04:05"))

	resp, err := client.Im.Message.Update(ctx, req)
	if err != nil {
		return "", err
	}

	// Record API request time
	apiTime := time.Since(startTime) - tokenTime
	log.Printf("[CardCreator] Card content update API request took: %d ms at %v", apiTime.Milliseconds(), time.Now().Format("15:04:05"))

	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", errors.New("failed to get message ID from response")
	}

	log.Printf("[CardCreator] Successfully updated card content with ID: %s at %v", *resp.Data.MessageId, time.Now().Format("15:04:05"))

	// Record total time
	totalTime := time.Since(startTime)
	log.Printf("[CardCreator] Total card content update took: %d ms at %v", totalTime.Milliseconds(), time.Now().Format("15:04:05"))

	return *resp.Data.MessageId, nil
}
