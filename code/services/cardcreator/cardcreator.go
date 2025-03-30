package cardcreator

import (
	"context"
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
	// Use Feishu API to create card entity
	client := c.config.GetLarkClient()
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType("interactive").
			Content(content).
			Build()).
		Build()
	resp, err := client.Im.Message.Create(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Data.MessageId, nil
}
