package handlers

import (
	"context"
	"encoding/json"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
)

func NewCardHandler(m *MessageHandler) CardHandlerFunc {
	handlers := []CardHandlerMeta{
		NewClearCardHandler,
		NewPicResolutionHandler,
		NewPicTextMoreHandler,
		NewPicModeChangeHandler,
	}

	return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
		var cardMsg CardMsg
		actionValue := cardAction.Action.Value
		actionValueJson, _ := json.Marshal(actionValue)
		json.Unmarshal(actionValueJson, &cardMsg)
		
		for _, handler := range handlers {
			h := handler(cardMsg, *m)
			i, err := h(ctx, cardAction)
			if err == ErrNextHandler {
				continue
			}
			return i, err
		}
		return nil, nil
	}
}
