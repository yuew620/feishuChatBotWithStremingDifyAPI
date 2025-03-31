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
	log.Printf("[Timing] Starting card entity creation")
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

	log.Printf("Creating card entity with URL: %s", req.ApiPath)

	// Record token fetch time
	tokenTime := time.Since(startTime)
	log.Printf("[Timing] Token fetch took: %d ms", tokenTime.Milliseconds())

	resp, err := client.Im.Message.Create(ctx, req)
	if err != nil {
		return "", err
	}

	// Record API request time
	apiTime := time.Since(startTime) - tokenTime
	log.Printf("[Timing] Card entity API request took: %d ms", apiTime.Milliseconds())

	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", errors.New("failed to get message ID from response")
	}

	log.Printf("Successfully created card entity with ID: %s", *resp.Data.MessageId)

	// Record total time
	totalTime := time.Since(startTime)
	log.Printf("[Timing] Total card entity creation took: %d ms", totalTime.Milliseconds())

	return *resp.Data.MessageId, nil
}

// UpdateCardContent updates the content of an existing card
func (c *CardCreator) UpdateCardContent(ctx context.Context, cardID string, content string) (string, error) {
	log.Printf("[Timing] Starting card content update")
	startTime := time.Now()

	// Use Feishu API to update card content
	client := c.config.GetLarkClient()
	req := larkim.NewUpdateMessageReqBuilder().
		MessageId(cardID).
		Body(larkim.NewUpdateMessageReqBodyBuilder().
			Content(content).
			Build()).
		Build()

	log.Printf("Updating card content with URL: %s", req.ApiPath)

	// Record token fetch time
	tokenTime := time.Since(startTime)
	log.Printf("[Timing] Token fetch took: %d ms", tokenTime.Milliseconds())

	resp, err := client.Im.Message.Update(ctx, req)
	if err != nil {
		return "", err
	}

	// Record API request time
	apiTime := time.Since(startTime) - tokenTime
	log.Printf("[Timing] Card content update API request took: %d ms", apiTime.Milliseconds())

	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", errors.New("failed to get message ID from response")
	}

	log.Printf("Successfully updated card content with ID: %s", *resp.Data.MessageId)

	// Record total time
	totalTime := time.Since(startTime)
	log.Printf("[Timing] Total card content update took: %d ms", totalTime.Milliseconds())

	return *resp.Data.MessageId, nil
}
