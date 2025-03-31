package handlers

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/services/config"
	"start-feishubot/services/core"
)

var globalConfig config.Config

// SetConfig sets the global configuration
func SetConfig(cfg config.Config) {
	globalConfig = cfg
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(
	sessionCache core.SessionCache,
	cardCreator core.CardCreator,
	msgCache core.MessageCache,
	aiProvider core.AIProvider,
) *MessageHandler {
	return &MessageHandler{
		sessionCache: sessionCache,
		cardCreator: cardCreator,
		msgCache:    msgCache,
		dify:        aiProvider,
	}
}

// msgReceivedHandler handles received messages
func (m *MessageHandler) msgReceivedHandler(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	return handleMessage(ctx, event, m)
}

// cardHandler handles card actions
func (m *MessageHandler) cardHandler(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
	// Parse card message
	var cardMsg CardMsg
	contentBytes, err := json.Marshal(cardAction.Action.Value)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(contentBytes, &cardMsg); err != nil {
		return nil, err
	}

	// Get handler for card kind
	handler := GetCardHandler(cardMsg, m)
	if handler == nil {
		return nil, nil
	}

	// Handle card action
	return handler(ctx, cardAction)
}

// judgeIfMentionMe checks if the bot is mentioned
func (m *MessageHandler) judgeIfMentionMe(mention []*larkim.MentionEvent) bool {
	if len(mention) != 1 {
		return false
	}
	return true
}

// Handler handles HTTP requests
func Handler(c *gin.Context) error {
	// Get event type
	var event struct {
		Type string `json:"type"`
	}
	if err := c.ShouldBindJSON(&event); err != nil {
		return err
	}

	// Handle URL verification
	if event.Type == "url_verification" {
		body, err := c.GetRawData()
		if err != nil {
			return err
		}

		result, err := VerifyURL(body, globalConfig)
		if err != nil {
			return err
		}

		c.JSON(200, result)
		return nil
	}

	// Handle other events
	return nil
}
