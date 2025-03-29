package handlers

import (
	"context"
	
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	"start-feishubot/services"
)

func NewClearCardHandler(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
	return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
		if cardMsg.Kind == ClearCardKind {
			newCard, err := CommonProcessClearCache(ctx, cardMsg, m.sessionCache)
			if err != nil {
				return nil, err
			}
			return newCard, nil
		}
		return nil, ErrNextHandler
	}
}

func CommonProcessClearCache(ctx context.Context, msg CardMsg, session services.SessionServiceCacheInterface) (interface{}, error) {
	if msg.Value == "1" {
		session.Clear(msg.SessionId)
		newCard, _ := newSendCard(
			withHeader("🧹 已清除上下文", larkcard.TemplateGreen),
			withMainMd("已清除此话题的上下文信息"),
			withNote("我们可以开始一个全新的话题，期待和您聊天。如果您有其他问题或者想要讨论的话题，请告诉我哦"),
		)
		return newCard, nil
	}
	if msg.Value == "0" {
		newCard, _ := newSendCard(
			withHeader("️🎒 机器人提醒", larkcard.TemplateGreen),
			withMainMd("依旧保留此话题的上下文信息"),
			withNote("我们可以继续探讨这个话题,期待和您聊天。如果您有其他问题或者想要讨论的话题，请告诉我哦"),
		)
		return newCard, nil
	}
	return nil, nil
}
