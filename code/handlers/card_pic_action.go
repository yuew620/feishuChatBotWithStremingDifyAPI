package handlers

import (
	"context"
	"fmt"
	
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	"start-feishubot/services"
)

// NewPicResolutionHandler handles picture resolution changes
func NewPicResolutionHandler(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
	return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
		if cardMsg.Kind == PicResolutionKind {
			newCard, err := CommonProcessPicResolution(ctx, cardMsg, m.sessionCache)
			if err != nil {
				return nil, err
			}
			return newCard, nil
		}
		return nil, ErrNextHandler
	}
}

// NewPicModeChangeHandler handles picture mode changes
func NewPicModeChangeHandler(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
	return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
		if cardMsg.Kind == PicModeChangeKind {
			newCard, err := CommonProcessPicModeChange(ctx, cardMsg, m.sessionCache)
			if err != nil {
				return nil, err
			}
			return newCard, nil
		}
		return nil, ErrNextHandler
	}
}

// NewPicTextMoreHandler handles requests for more picture text
func NewPicTextMoreHandler(cardMsg CardMsg, m *MessageHandler) CardHandlerFunc {
	return func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
		if cardMsg.Kind == PicTextMoreKind {
			// Get the resolution from session cache
			resolution := m.sessionCache.GetPicResolution(cardMsg.SessionId)
			
			// Create a new card with the resolution
			newCard, _ := newSendCard(
				withHeader("🎨 图片生成", larkcard.TemplateBlue),
				withMainMd(fmt.Sprintf("正在生成图片，分辨率: %s", resolution)),
				withNote("请稍等片刻，图片生成中..."),
			)
			return newCard, nil
		}
		return nil, ErrNextHandler
	}
}

// CommonProcessPicResolution processes picture resolution changes
func CommonProcessPicResolution(ctx context.Context, msg CardMsg, session services.SessionServiceCacheInterface) (interface{}, error) {
	// Set the resolution in the session cache
	resolution, ok := msg.Value.(string)
	if !ok {
		resolution = "512x512" // Default resolution
	}
	
	session.SetPicResolution(msg.SessionId, resolution)
	
	// Create a new card with the resolution
	newCard, _ := newSendCard(
		withHeader("🎨 分辨率已设置", larkcard.TemplateGreen),
		withMainMd(fmt.Sprintf("图片分辨率已设置为: %s", resolution)),
		withNote("您可以继续发送文本来生成图片"),
	)
	return newCard, nil
}

// CommonProcessPicModeChange processes picture mode changes
func CommonProcessPicModeChange(ctx context.Context, msg CardMsg, session services.SessionServiceCacheInterface) (interface{}, error) {
	// Set the mode in the session cache
	mode, ok := msg.Value.(string)
	if !ok {
	mode = string(services.ModePicCreate) // Default mode
}

session.SetMode(msg.SessionId, services.SessionMode(mode))
	
	// Create a new card with the mode
	newCard, _ := newSendCard(
		withHeader("🎨 模式已切换", larkcard.TemplateGreen),
		withMainMd(fmt.Sprintf("图片模式已切换为: %s", mode)),
		withNote("您可以继续发送文本来生成图片"),
	)
	return newCard, nil
}
