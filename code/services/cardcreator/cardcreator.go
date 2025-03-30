package cardcreator

import (
	"context"
	"start-feishubot/services/feishu"
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
	resp, err := client.Im.CreateMessage(ctx, content)
	if err != nil {
		return "", err
	}
	return resp.MessageId, nil
}
