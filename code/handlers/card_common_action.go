package handlers

import (
	"context"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
)

type CardHandlerMap map[CardKind]CardHandlerMeta

func GetCardHandler(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
	if handler, ok := cardHandlerMap[cardMsg.Kind]; ok {
		return handler(cardMsg, m)
	}
	return nil
}

var cardHandlerMap = CardHandlerMap{
	ClearCardKind: func(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
		return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
			return CommonProcessClearCache(ctx, cardAction, m.sessionCache, cardMsg.SessionId, cardMsg.MsgId)
		}
	},
	PicResolutionKind: func(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
		return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
			return CommonProcessPicResolution(ctx, cardAction, m.sessionCache, cardMsg.SessionId, cardMsg.MsgId)
		}
	},
	PicModeChangeKind: func(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
		return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
			return CommonProcessPicModeChange(ctx, cardAction, m.sessionCache, cardMsg.SessionId, cardMsg.MsgId)
		}
	},
	RoleTagsChooseKind: func(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
		return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
			return CommonProcessRoleTag(ctx, cardAction, m.sessionCache, cardMsg.SessionId, cardMsg.MsgId)
		}
	},
	RoleChooseKind: func(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
		return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
			return CommonProcessRole(ctx, cardAction, m.sessionCache, cardMsg.SessionId, cardMsg.MsgId)
		}
	},
}
