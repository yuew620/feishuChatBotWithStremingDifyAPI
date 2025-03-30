package handlers

import (
	"context"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type EventAction interface {
	Execute(a *ActionInfo) bool
}

type CommonMessageAction struct {
	ctx     *context.Context
	info    *MsgInfo
	handler *MessageHandler
}

func (a *CommonMessageAction) Execute(info *ActionInfo) bool {
	if info.info.msgId == nil {
		return false
	}
	if a.handler.msgCache.IfProcessed(*info.info.msgId) {
		return false
	}
	a.handler.msgCache.TagProcessed(*info.info.msgId)
	return true
}

func NewCommonMessageAction(ctx *context.Context, info *MsgInfo, handler *MessageHandler) *CommonMessageAction {
	return &CommonMessageAction{
		ctx:     ctx,
		info:    info,
		handler: handler,
	}
}

func NewActionInfo(ctx *context.Context, info *MsgInfo, handler *MessageHandler) *ActionInfo {
	return &ActionInfo{
		ctx:     ctx,
		info:    info,
		handler: handler,
	}
}

func NewMsgInfo(msg *larkim.P2MessageReceiveV1) *MsgInfo {
	return &MsgInfo{
		handlerType: judgeChatType(msg),
		msgType:     *msg.Event.Message.MessageType,
		msgId:       msg.Event.Message.MessageId,
		chatId:      *msg.Event.Message.ChatId,
		userId:      *msg.Event.Sender.SenderId.UserId,
		mention:     msg.Event.Message.Mentions,
	}
}
