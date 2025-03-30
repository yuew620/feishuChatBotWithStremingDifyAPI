package handlers

import (
	"context"
	"encoding/json"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/services/core"
)

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
