package handlers

import (
	"context"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	"start-feishubot/initialization"
	"start-feishubot/services"
	"start-feishubot/services/openai"
)

func NewPicResolutionHandler(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
	return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
		if cardMsg.Kind == PicResolutionKind {
			CommonProcessPicResolution(ctx, cardMsg, cardAction, m.sessionCache)
			return nil, nil
		}
		return nil, ErrNextHandler
	}
}

func NewPicModeChangeHandler(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
	return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
		if cardMsg.Kind == PicModeChangeKind {
			newCard, err, done := CommonProcessPicModeChange(ctx, cardMsg, m.sessionCache)
			if done {
				return newCard, err
			}
			return nil, nil
		}
		return nil, ErrNextHandler
	}
}

func NewPicTextMoreHandler(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
	return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
		if cardMsg.Kind == PicTextMoreKind {
			go func() {
				m.CommonProcessPicMore(ctx, cardMsg)
			}()
			return nil, nil
		}
		return nil, ErrNextHandler
	}
}

func CommonProcessPicResolution(ctx context.Context, msg CardMsg,
	cardAction *larkcard.CardAction,
	cache services.SessionServiceCacheInterface) {
	option := cardAction.Action.Option
	cache.SetPicResolution(msg.SessionId, option)
	replyMsg(ctx, "已更新图片分辨率为"+option,
		&msg.MsgId)
}

func (m *MessageHandler) CommonProcessPicMore(ctx context.Context, msg CardMsg) {
	resolution := m.sessionCache.GetPicResolution(msg.SessionId)
	question := msg.Value.(string)
	config := initialization.GetConfig()
	gpt := openai.NewChatGPT(config)
	bs64, _ := gpt.GenerateOneImage(question, resolution)
	replayImageCardByBase64(ctx, bs64, &msg.MsgId,
		&msg.SessionId, question)
}

func CommonProcessPicModeChange(ctx context.Context, cardMsg CardMsg,
	session services.SessionServiceCacheInterface) (
	interface{}, error, bool) {
	if cardMsg.Value == "1" {
		sessionId := cardMsg.SessionId
		session.Clear(sessionId)
		session.SetMode(sessionId,
			services.ModePicCreate)
		session.SetPicResolution(sessionId, "256x256")

		newCard, _ :=
			newSendCard(
				withHeader("🖼️ 已进入图片创作模式", larkcard.TemplateBlue),
				withPicResolutionBtn(&sessionId),
				withNote("提醒：回复文本或图片，让AI生成相关的图片。"))
		return newCard, nil, true
	}
	if cardMsg.Value == "0" {
		newCard, _ := newSendCard(
			withHeader("️🎒 机器人提醒", larkcard.TemplateGreen),
			withMainMd("依旧保留此话题的上下文信息"),
			withNote("我们可以继续探讨这个话题,期待和您聊天。如果您有其他问题或者想要讨论的话题，请告诉我哦"),
		)
		return newCard, nil, true
	}
	return nil, nil, false
}
