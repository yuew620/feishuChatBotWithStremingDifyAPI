package handlers

import (
	"context"
	"fmt"
	"strings"

	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/initialization"
	"start-feishubot/services"
	"start-feishubot/services/dify"
)

// NewMessageHandler creates a new MessageHandler instance
func NewMessageHandler(sessionCache SessionServiceCacheInterface, cardCreator CardCreator, msgCache MessageCacheInterface, dify *dify.DifyClient) *MessageHandler {
	return &MessageHandler{
		sessionCache: sessionCache,
		cardCreator:  cardCreator,
		msgCache:     msgCache,
		dify:        dify,
	}
}

// 判断是否提到我
func (m *MessageHandler) judgeIfMentionMe(mention []*larkim.MentionEvent) bool {
	if len(mention) != 1 {
		return false
	}
	return true
}

// msgReceivedHandler handles received messages
func (m *MessageHandler) msgReceivedHandler(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	handlerType := judgeChatType(event)
	if handlerType == OtherHandler {
		return nil
	}
	
	content := event.Event.Message.Content
	if content == nil {
		return nil
	}
	
	msgId := event.Event.Message.MessageId
	if msgId == nil {
		return nil
	}
	
	chatId := event.Event.Message.ChatId
	if chatId == nil {
		return nil
	}
	
	userId := event.Event.Sender.SenderId.UserId
	if userId == nil {
		return nil
	}
	
	sessionId := fmt.Sprintf("%s-%s", *chatId, *userId)
	mention := event.Event.Message.Mentions
	
	qParsed := strings.TrimSpace(*content)
	
	actionInfo := &ActionInfo{
		ctx: &ctx,
		info: &MsgInfo{
			handlerType: handlerType,
			msgType:     *event.Event.Message.MessageType,
			msgId:       msgId,
			chatId:      *chatId,
			userId:      *userId,
			qParsed:     qParsed,
			sessionId:   &sessionId,
			mention:     mention,
		},
		handler: m,
	}
	
	return m.ExecuteChain(actionInfo)
}

// cardHandler handles card actions
func (m *MessageHandler) cardHandler(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
	if cardAction == nil {
		return nil, fmt.Errorf("card action is nil")
	}

	value, ok := cardAction.Action.Value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid action value type")
	}

	methodName, ok := value["key"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid key type")
	}

	sessionId, ok := value["sessionId"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid sessionId type")
	}

	messageId, ok := value["messageId"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid messageId type")
	}

	cardMsg := CardMsg{
		Kind:      CardKind(methodName),
		SessionId: sessionId,
		MsgId:     messageId,
		Value:     value["value"],
	}

	handlers := []CardHandlerFunc{
		NewPicResolutionHandler(cardMsg, m),
		NewPicModeChangeHandler(cardMsg, m),
		NewPicTextMoreHandler(cardMsg, m),
		NewClearCardHandler(cardMsg, m),
	}

	for _, handler := range handlers {
		resp, err := handler(ctx, cardAction)
		if err == nil {
			return resp, nil
		}
		if err != ErrNextHandler {
			return nil, err
		}
	}

	return nil, fmt.Errorf("no handler found for method: %s", methodName)
}

// ExecuteChain processes the action through the responsibility chain
func (m *MessageHandler) ExecuteChain(a *ActionInfo) error {
	actions := []Action{
		&ProcessedUniqueAction{},
		&ProcessMentionAction{},
		&EmptyAction{},
		&ClearAction{},
		&RolePlayAction{},
		&HelpAction{},
		&BalanceAction{},
		&RoleListAction{},
	}
	
	if !chain(a, actions...) {
		return nil
	}
	
	return nil
}

// 责任链
func chain(data *ActionInfo, actions ...Action) bool {
	for _, v := range actions {
		if !v.Execute(data) {
			return false
		}
	}
	return true
}
