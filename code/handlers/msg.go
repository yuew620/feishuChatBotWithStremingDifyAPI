package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"github.com/google/uuid"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
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
	
	// 构建请求体
	reqBody := map[string]interface{}{
		"uuid":     uuid.New().String(), // 使用UUID保证幂等性
		"content":  content,
		"sequence": getNextSequence(), // 使用原子计数器获取序列号
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// 构建请求URL
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/cardkit/v1/cards/%s/elements/%s/content", cardId, elementId)
	
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.GetTenantAccessToken()))

	// 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
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

// withMainMd 用于生成markdown消息体
func withMainMd(msg string) larkcard.MessageCardElement {
	msg, i := processMessage(msg)
	msg = processNewLine(msg)
	if i != nil {
		return nil
	}
	mainElement := larkcard.NewMessageCardDiv().
		Fields([]*larkcard.MessageCardField{larkcard.NewMessageCardField().
			Text(larkcard.NewMessageCardLarkMd().
				Content(msg).
				Build()).
			IsShort(true).
			Build()}).
		Build()
	return mainElement
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

// withHeader 用于生成消息头
func withHeader(title string, color string) *larkcard.MessageCardHeader {
	if title == "" {
		title = "🤖️机器人提醒"
	}
	header := larkcard.NewMessageCardHeader().
		Template(color).
		Title(larkcard.NewMessageCardPlainText().
			Content(title).
			Build()).
		Build()
	return header
}

// withNote 用于生成纯文本脚注
func withNote(note string) larkcard.MessageCardElement {
	noteElement := larkcard.NewMessageCardNote().
		Elements([]larkcard.MessageCardNoteElement{larkcard.NewMessageCardPlainText().
			Content(note).
			Build()}).
		Build()
	return noteElement
}

// withPicResolutionBtn 用于生成图片分辨率按钮
func withPicResolutionBtn(sessionID *string) larkcard.MessageCardElement {
	cancelMenu := newMenu("默认分辨率",
		map[string]interface{}{
			"value":     "0",
			"kind":      PicResolutionKind,
			"sessionId": *sessionID,
			"msgId":     *sessionID,
		},
		MenuOption{
			label: "256x256",
			value: "256x256",
		},
		MenuOption{
			label: "512x512",
			value: "512x512",
		},
		MenuOption{
			label: "1024x1024",
			value: "1024x1024",
		},
	)

	actions := larkcard.NewMessageCardAction().
		Actions([]larkcard.MessageCardActionElement{cancelMenu}).
		Layout(larkcard.MessageCardActionLayoutFlow.Ptr()).
		Build()
	return actions
}

// replyMsg 用于回复普通文本消息
func replyMsg(ctx context.Context, msg string, msgId *string) error {
	msg, i := processMessage(msg)
	if i != nil {
		return i
	}
	client := initialization.GetLarkClient()
	content := larkim.NewTextMsgBuilder().
		Text(msg).
		Build()

	resp, err := client.Im.Message.Reply(ctx, larkim.NewReplyMessageReqBuilder().
		MessageId(*msgId).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			Uuid(uuid.New().String()).
			Content(content).
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

// replayImageCardByBase64 用于回复图片卡片
func replayImageCardByBase64(ctx context.Context, base64Str string, msgId *string, sessionId *string, question string) error {
	imageKey, err := uploadImage(base64Str)
	if err != nil {
		return err
	}
	err = sendImageCard(ctx, *imageKey, msgId, sessionId, question)
	if err != nil {
		return err
	}
	return nil
}

// withSplitLine 用于生成分割线
func withSplitLine() larkcard.MessageCardElement {
	splitLine := larkcard.NewMessageCardHr().
		Build()
	return splitLine
}

// withImageDiv 用于生成图片元素
func withImageDiv(imageKey string) larkcard.MessageCardElement {
	imageElement := larkcard.NewMessageCardImage().
		ImgKey(imageKey).
		Alt(larkcard.NewMessageCardPlainText().Content("").
			Build()).
		Preview(true).
		Mode(larkcard.MessageCardImageModelCropCenter).
		CompactWidth(true).
		Build()
	return imageElement
}

// withMdAndExtraBtn 用于生成带有额外按钮的消息体
func withMdAndExtraBtn(msg string, btn *larkcard.MessageCardEmbedButton) larkcard.MessageCardElement {
	msg, i := processMessage(msg)
	msg = processNewLine(msg)
	if i != nil {
		return nil
	}
	mainElement := larkcard.NewMessageCardDiv().
		Fields([]*larkcard.MessageCardField{larkcard.NewMessageCardField().
			Text(larkcard.NewMessageCardLarkMd().
				Content(msg).
				Build()).
			IsShort(true).
			Build()}).
		Extra(btn).
		Build()
	return mainElement
}

// withOneBtn 用于生成单个按钮
func withOneBtn(btn *larkcard.MessageCardEmbedButton) larkcard.MessageCardElement {
	actions := larkcard.NewMessageCardAction().
		Actions([]larkcard.MessageCardActionElement{btn}).
		Layout(larkcard.MessageCardActionLayoutFlow.Ptr()).
		Build()
	return actions
}

// sendImageCard 用于发送图片卡片
func sendImageCard(ctx context.Context, imageKey string, msgId *string, sessionId *string, question string) error {
	newCard, _ := newSimpleSendCard(
		withImageDiv(imageKey),
		withSplitLine(),
		withOneBtn(newBtn("再来一张", map[string]interface{}{
			"value":     question,
			"kind":      PicTextMoreKind,
			"chatType":  UserChatType,
			"msgId":     *msgId,
			"sessionId": *sessionId,
		}, larkcard.MessageCardButtonTypePrimary)),
	)
	return replyCard(ctx, msgId, newCard)
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
