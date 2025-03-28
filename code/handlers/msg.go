package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"github.com/google/uuid"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkcardkit "github.com/larksuite/oapi-sdk-go/v3/service/cardkit/v1"
	"start-feishubot/initialization"
	"start-feishubot/services/openai"
)

type CardKind string
type CardChatType string

var (
	ClearCardKind      = CardKind("clear")            // 清空上下文
	PicModeChangeKind  = CardKind("pic_mode_change")  // 切换图片创作模式
	PicResolutionKind  = CardKind("pic_resolution")   // 图片分辨率调整
	PicTextMoreKind    = CardKind("pic_text_more")    // 重新根据文本生成图片
	PicVarMoreKind     = CardKind("pic_var_more")     // 变量图片
	RoleTagsChooseKind = CardKind("role_tags_choose") // 内置角色所属标签选择
	RoleChooseKind     = CardKind("role_choose")      // 内置角色选择
)

var (
	GroupChatType = CardChatType("group")
	UserChatType  = CardChatType("personal")
)

// 全局序列号计数器
var sequenceCounter int64

// 获取下一个序列号
func getNextSequence() int64 {
	return atomic.AddInt64(&sequenceCounter, 1)
}

// 流式更新文本内容
func streamUpdateText(ctx context.Context, cardId string, elementId string, content string) error {
	client := initialization.GetLarkClient()
	
	// 创建请求对象
	req := larkcardkit.NewContentCardElementReqBuilder().
		CardId(cardId).
		ElementId(elementId).
		Body(larkcardkit.NewContentCardElementReqBodyBuilder().
			Uuid(uuid.New().String()). // 使用UUID保证幂等性
			Content(content).
			Sequence(getNextSequence()). // 使用原子计数器获取序列号
			Build()).
		Build()

	// 发起请求
	resp, err := client.Cardkit.V1.CardElement.Content(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to update card content: %w", err)
	}

	// 服务端错误处理
	if !resp.Success() {
		return fmt.Errorf("API error: %s", resp.Msg)
	}

	return nil
}

type CardMsg struct {
	Kind      CardKind
	ChatType  CardChatType
	Value     interface{}
	SessionId string
	MsgId     string
}

type MenuOption struct {
	value string
	label string
}

func replyCard(ctx context.Context, msgId *string, cardContent string) error {
	client := initialization.GetLarkClient()
	resp, err := client.Im.Message.Reply(ctx, larkim.NewReplyMessageReqBuilder().
		MessageId(*msgId).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			Uuid(uuid.New().String()).
			Content(cardContent).
			Build()).
		Build())

	if err != nil {
		fmt.Println(err)
		return err
	}

	if !resp.Success() {
		fmt.Println(resp.Code, resp.Msg, resp.RequestId())
		return errors.New(resp.Msg)
	}
	return nil
}

func replyCardWithBackId(ctx context.Context, msgId *string, cardContent string) (*string, error) {
	client := initialization.GetLarkClient()
	resp, err := client.Im.Message.Reply(ctx, larkim.NewReplyMessageReqBuilder().
		MessageId(*msgId).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			Uuid(uuid.New().String()).
			Content(cardContent).
			Build()).
		Build())

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if !resp.Success() {
		fmt.Println(resp.Code, resp.Msg, resp.RequestId())
		return nil, errors.New(resp.Msg)
	}

	return resp.Data.MessageId, nil
}

func newSendCard(header *larkcard.MessageCardHeader, elements ...larkcard.MessageCardElement) (string, error) {
	config := &larkcard.MessageCardConfig{
		WideScreenMode: false,
		EnableForward:  true,
		UpdateMulti:    true,
		StreamingMode:  true, // 启用流式更新模式
	}
	var aElementPool []larkcard.MessageCardElement
	for _, element := range elements {
		aElementPool = append(aElementPool, element)
	}
	cardContent, err := larkcard.NewMessageCard().
		Config(config).
		Header(header).
		Elements(aElementPool).
		String()
	return cardContent, err
}

func newSendCardWithOutHeader(elements ...larkcard.MessageCardElement) (string, error) {
	config := &larkcard.MessageCardConfig{
		WideScreenMode: false,
		EnableForward:  true,
		UpdateMulti:    true,
		StreamingMode:  true, // 启用流式更新模式
	}
	var aElementPool []larkcard.MessageCardElement
	for _, element := range elements {
		aElementPool = append(aElementPool, element)
	}
	cardContent, err := larkcard.NewMessageCard().
		Config(config).
		Elements(aElementPool).
		String()
	return cardContent, err
}

func newSimpleSendCard(elements ...larkcard.MessageCardElement) (string, error) {
	config := &larkcard.MessageCardConfig{
		WideScreenMode: false,
		EnableForward:  true,
		UpdateMulti:    true,
		StreamingMode:  true, // 启用流式更新模式
	}
	var aElementPool []larkcard.MessageCardElement
	for _, element := range elements {
		aElementPool = append(aElementPool, element)
	}
	cardContent, err := larkcard.NewMessageCard().
		Config(config).
		Elements(aElementPool).
		String()
	return cardContent, err
}

// withMainText 用于生成纯文本消息体
func withMainText(msg string) larkcard.MessageCardElement {
	msg, i := processMessage(msg)
	msg = cleanTextBlock(msg)
	if i != nil {
		return nil
	}
	mainElement := larkcard.NewMessageCardDiv().
		Fields([]*larkcard.MessageCardField{larkcard.NewMessageCardField().
			Text(larkcard.NewMessageCardPlainText().
				Content(msg).
				Build()).
			IsShort(false).
			Build()}).
		ElementId("content_block"). // 为流式更新设置element_id
		Build()
	return mainElement
}

// 更新卡片文本内容
func updateTextCard(ctx context.Context, msg string, cardId *string) error {
	// 使用流式更新API
	err := streamUpdateText(ctx, *cardId, "content_block", msg)
	if err != nil {
		return fmt.Errorf("failed to stream update text: %w", err)
	}
	return nil
}

[其余函数保持不变...]
