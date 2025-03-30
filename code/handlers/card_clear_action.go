package handlers

import (
	"context"
	"encoding/json"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	"start-feishubot/services/core"
)

func CommonProcessClearCache(
	ctx context.Context,
	cardAction *larkcard.CardAction,
	sessionCache core.SessionCache,
	userId string,
	messageId string,
) (interface{}, error) {
	contentBytes, err := json.Marshal(cardAction.Action.Value)
	if err != nil {
		return nil, err
	}

	var cardMsg CardMsg
	err = json.Unmarshal(contentBytes, &cardMsg)
	if err != nil {
		return nil, err
	}

	sessionCache.Clear(cardMsg.SessionId)
	return nil, nil
}
