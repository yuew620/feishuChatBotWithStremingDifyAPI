package handlers

import (
	"context"
	"encoding/json"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	"start-feishubot/services/core"
)

func CommonProcessRoleTag(
	ctx context.Context,
	cardAction *larkcard.CardAction,
	sessionCache core.SessionCache,
	userId string,
	messageId string,
) (interface{}, error) {
	content := cardAction.Action.Value
	var cardMsg CardMsg
	err := json.Unmarshal([]byte(content), &cardMsg)
	if err != nil {
		return nil, err
	}

	// Process role tag
	return nil, nil
}

func CommonProcessRole(
	ctx context.Context,
	cardAction *larkcard.CardAction,
	sessionCache core.SessionCache,
	userId string,
	messageId string,
) (interface{}, error) {
	content := cardAction.Action.Value
	var cardMsg CardMsg
	err := json.Unmarshal([]byte(content), &cardMsg)
	if err != nil {
		return nil, err
	}

	// Process role
	return nil, nil
}
