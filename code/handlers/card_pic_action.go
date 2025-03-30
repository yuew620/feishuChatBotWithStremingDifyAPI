package handlers

import (
	"context"
	"encoding/json"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	"start-feishubot/services/core"
)

func CommonProcessPicResolution(
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

	sessionCache.SetPicResolution(cardMsg.SessionId, cardMsg.Value.(string))
	return nil, nil
}

func CommonProcessPicModeChange(
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

	mode := core.ModePicCreate
	if cardMsg.Value == "vary" {
		mode = core.ModePicVary
	}
	sessionCache.SetMode(cardMsg.SessionId, mode)
	return nil, nil
}
