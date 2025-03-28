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

// 获取tenant_access_token
func getTenantAccessToken(ctx context.Context) (string, error) {
	client := initialization.GetLarkClient()
	config := initialization.GetConfig()
	
	// 构建请求体
	reqBody := map[string]interface{}{
		"app_id":     config.FeishuAppId,
		"app_secret": config.FeishuAppSecret,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// 构建请求URL
	url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	if result.Code != 0 {
		return "", fmt.Errorf("API error: code=%d, msg=%s", result.Code, result.Msg)
	}
	
	return result.TenantAccessToken, nil
}

// 流式更新文本内容
func streamUpdateText(ctx context.Context, cardId string, elementId string, content string) error {
	// 获取tenant_access_token
	token, err := getTenantAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant_access_token: %w", err)
	}
	
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

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
	// 使用Builder模式创建配置
	config := larkcard.NewMessageCardConfig().
		WideScreenMode(false).
		Build()
	
	var aElementPool []larkcard.MessageCardElement
	for _, element := range elements {
		aElementPool = append(aElementPool, element)
	}
	
	// 添加额外的配置到JSON
	cardObj := larkcard.NewMessageCard().
		Config(config).
		Header(header).
		Elements(aElementPool)
	
	// 获取JSON字符串
	cardStr, err := cardObj.String()
	if err != nil {
		return "", err
	}
	
	// 解析JSON以添加额外的配置
	var cardJSON map[string]interface{}
	if err := json.Unmarshal([]byte(cardStr), &cardJSON); err != nil {
		return "", err
	}
	
	// 添加流式更新和多次更新配置
	if configObj, ok := cardJSON["config"].(map[string]interface{}); ok {
		configObj["update_multi"] = true
		configObj["streaming_mode"] = true
	}
	
	// 重新序列化
	modifiedJSON, err := json.Marshal(cardJSON)
	if err != nil {
		return "", err
	}
	
	return string(modifiedJSON), nil
}

func newSendCardWithOutHeader(elements ...larkcard.MessageCardElement) (string, error) {
	// 使用Builder模式创建配置
	config := larkcard.NewMessageCardConfig().
		WideScreenMode(false).
		Build()
	
	var aElementPool []larkcard.MessageCardElement
	for _, element := range elements {
		aElementPool = append(aElementPool, element)
	}
	
	// 添加额外的配置到JSON
	cardObj := larkcard.NewMessageCard().
		Config(config).
		Elements(aElementPool)
	
	// 获取JSON字符串
	cardStr, err := cardObj.String()
	if err != nil {
		return "", err
	}
	
	// 解析JSON以添加额外的配置
	var cardJSON map[string]interface{}
	if err := json.Unmarshal([]byte(cardStr), &cardJSON); err != nil {
		return "", err
	}
	
	// 添加流式更新和多次更新配置
	if configObj, ok := cardJSON["config"].(map[string]interface{}); ok {
		configObj["update_multi"] = true
		configObj["streaming_mode"] = true
	}
	
	// 重新序列化
	modifiedJSON, err := json.Marshal(cardJSON)
	if err != nil {
		return "", err
	}
	
	return string(modifiedJSON), nil
}

func newSimpleSendCard(elements ...larkcard.MessageCardElement) (string, error) {
	// 使用Builder模式创建配置
	config := larkcard.NewMessageCardConfig().
		WideScreenMode(false).
		Build()
	
	var aElementPool []larkcard.MessageCardElement
	for _, element := range elements {
		aElementPool = append(aElementPool, element)
	}
	
	// 添加额外的配置到JSON
	cardObj := larkcard.NewMessageCard().
		Config(config).
		Elements(aElementPool)
	
	// 获取JSON字符串
	cardStr, err := cardObj.String()
	if err != nil {
		return "", err
	}
	
	// 解析JSON以添加额外的配置
	var cardJSON map[string]interface{}
	if err := json.Unmarshal([]byte(cardStr), &cardJSON); err != nil {
		return "", err
	}
	
	// 添加流式更新和多次更新配置
	if configObj, ok := cardJSON["config"].(map[string]interface{}); ok {
		configObj["update_multi"] = true
		configObj["streaming_mode"] = true
	}
	
	// 重新序列化
	modifiedJSON, err := json.Marshal(cardJSON)
	if err != nil {
		return "", err
	}
	
	return string(modifiedJSON), nil
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

// newBtn 用于创建按钮
func newBtn(content string, value map[string]interface{}, typename larkcard.MessageCardButtonType) *larkcard.MessageCardEmbedButton {
	btn := larkcard.NewMessageCardEmbedButton().
		Type(typename).
		Value(value).
		Text(larkcard.NewMessageCardPlainText().
			Content(content).
			Build())
	return btn
}

// withClearDoubleCheckBtn 用于生成清除确认按钮
func withClearDoubleCheckBtn(sessionID *string) larkcard.MessageCardElement {
	confirmBtn := newBtn("确认清除", map[string]interface{}{
		"value":     "1",
		"kind":      ClearCardKind,
		"chatType":  UserChatType,
		"sessionId": *sessionID,
	}, larkcard.MessageCardButtonTypeDanger,
	)
	cancelBtn := newBtn("我再想想", map[string]interface{}{
		"value":     "0",
		"kind":      ClearCardKind,
		"sessionId": *sessionID,
		"chatType":  UserChatType,
	},
		larkcard.MessageCardButtonTypeDefault)

	actions := larkcard.NewMessageCardAction().
		Actions([]larkcard.MessageCardActionElement{confirmBtn, cancelBtn}).
		Layout(larkcard.MessageCardActionLayoutBisected.Ptr()).
		Build()

	return actions
}

// withRoleTagsBtn 用于生成角色标签按钮
func withRoleTagsBtn(sessionID *string, tags ...string) larkcard.MessageCardElement {
	var menuOptions []MenuOption

	for _, tag := range tags {
		menuOptions = append(menuOptions, MenuOption{
			label: tag,
			value: tag,
		})
	}
	cancelMenu := newMenu("选择角色分类",
		map[string]interface{}{
			"value":     "0",
			"kind":      RoleTagsChooseKind,
			"sessionId": *sessionID,
			"msgId":     *sessionID,
		},
		menuOptions...,
	)

	actions := larkcard.NewMessageCardAction().
		Actions([]larkcard.MessageCardActionElement{cancelMenu}).
		Layout(larkcard.MessageCardActionLayoutFlow.Ptr()).
		Build()
	return actions
}

// withRoleBtn 用于生成角色按钮
func withRoleBtn(sessionID *string, titles ...string) larkcard.MessageCardElement {
	var menuOptions []MenuOption

	for _, tag := range titles {
		menuOptions = append(menuOptions, MenuOption{
			label: tag,
			value: tag,
		})
	}
	cancelMenu := newMenu("查看内置角色",
		map[string]interface{}{
			"value":     "0",
			"kind":      RoleChooseKind,
			"sessionId": *sessionID,
			"msgId":     *sessionID,
		},
		menuOptions...,
	)

	actions := larkcard.NewMessageCardAction().
		Actions([]larkcard.MessageCardActionElement{cancelMenu}).
		Layout(larkcard.MessageCardActionLayoutFlow.Ptr()).
		Build()
	return actions
}

// PatchCard 用于更新卡片
func PatchCard(ctx context.Context, msgId *string, cardContent string) error {
	client := initialization.GetLarkClient()
	resp, err := client.Im.Message.Patch(ctx, larkim.NewPatchMessageReqBuilder().
		MessageId(*msgId).
		Body(larkim.NewPatchMessageReqBodyBuilder().
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

// newMenu 用于创建下拉菜单
func newMenu(placeHolder string, value map[string]interface{}, options ...MenuOption) larkcard.MessageCardActionElement {
	var selectOptions []*larkcard.MessageCardEmbedSelectOption
	for _, option := range options {
		selectOptions = append(selectOptions, larkcard.NewMessageCardEmbedSelectOption().
			Value(option.value).
			Text(larkcard.NewMessageCardPlainText().
				Content(option.label).
				Build()).
			Build())
	}

	menu := larkcard.NewMessageCardEmbedSelectStaticMenu().
		Options(selectOptions).
		Placeholder(larkcard.NewMessageCardPlainText().
			Content(placeHolder).
			Build()).
		Value(value).
		Build()

	return menu
}

// uploadImage 用于上传图片
func uploadImage(base64Str string) (*string, error) {
	client := initialization.GetLarkClient()

	// 解码Base64字符串
	imageBytes, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// 上传图片
	resp, err := client.Im.Image.Create(context.Background(),
		larkim.NewCreateImageReqBuilder().
			Body(larkim.NewCreateImageReqBodyBuilder().
				ImageType(larkim.ImageTypeMessage).
				Image(bytes.NewReader(imageBytes)).
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

	return resp.Data.ImageKey, nil
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

// 更新最终卡片
func updateFinalCard(ctx context.Context, msg string, msgId *string) error {
	newCard, _ := newSendCardWithOutHeader(
		withMainText(msg))
	err := PatchCard(ctx, msgId, newCard)
	if err != nil {
		return err
	}
	return nil
}

// 发送普通消息
func sendMsg(ctx context.Context, msg string, chatId *string) error {
	msg, i := processMessage(msg)
	if i != nil {
		return i
	}
	client := initialization.GetLarkClient()
	content := larkim.NewTextMsgBuilder().
		Text(msg).
		Build()

	resp, err := client.Im.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			ReceiveId(*chatId).
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

// 发送清除缓存确认卡片
func sendClearCacheCheckCard(ctx context.Context, sessionId *string, msgId *string) {
	newCard, _ := newSendCard(
		withHeader("🆑 机器人提醒", larkcard.TemplateBlue),
		withMainMd("您确定要清除对话上下文吗？"),
		withNote("请注意，这将开始一个全新的对话，您将无法利用之前话题的历史信息"),
		withClearDoubleCheckBtn(sessionId))
	replyCard(ctx, msgId, newCard)
}

// 发送系统指令卡片
func sendSystemInstructionCard(ctx context.Context, sessionId *string, msgId *string, content string) {
	newCard, _ := newSendCard(
		withHeader("🥷  已进入角色扮演模式", larkcard.TemplateIndigo),
		withMainText(content),
		withNote("请注意，这将开始一个全新的对话，您将无法利用之前话题的历史信息"))
	replyCard(ctx, msgId, newCard)
}

// 发送帮助卡片
func sendHelpCard(ctx context.Context, sessionId *string, msgId *string) {
	newCard, _ := newSendCard(
		withHeader("🎒需要帮助吗？", larkcard.TemplateBlue),
		withMainMd("**我是具备打字机效果的聊天机器人！**"),
		withSplitLine(),
		withMdAndExtraBtn(
			"** 🆑 清除话题上下文**\n文本回复 *清除* 或 */clear*",
			newBtn("立刻清除", map[string]interface{}{
				"value":     "1",
				"kind":      ClearCardKind,
				"chatType":  UserChatType,
				"sessionId": *sessionId,
			}, larkcard.MessageCardButtonTypeDanger)),
		withMainMd("🛖 **内置角色列表** \n"+" 文本回复 *角色列表* 或 */roles*"),
		withMainMd("🥷 **角色扮演模式**\n文本回复*角色扮演* 或 */system*+空格+角色信息"),
		withSplitLine(),
		withMainMd("🎒 **需要更多帮助**\n文本回复 *帮助* 或 */help*"),
	)
	replyCard(ctx, msgId, newCard)
}

// 发送余额卡片
func sendBalanceCard(ctx context.Context, msgId *string, balance openai.BalanceResponse) {
	newCard, _ := newSendCard(
		withHeader("🎰️ 余额查询", larkcard.TemplateBlue),
		withMainMd(fmt.Sprintf("总额度: %.2f$", balance.TotalGranted)),
		withMainMd(fmt.Sprintf("已用额度: %.2f$", balance.TotalUsed)),
		withMainMd(fmt.Sprintf("可用额度: %.2f$", balance.TotalAvailable)),
		withNote(fmt.Sprintf("有效期: %s - %s",
			balance.EffectiveAt.Format("2006-01-02 15:04:05"),
			balance.ExpiresAt.Format("2006-01-02 15:04:05"))),
	)
	replyCard(ctx, msgId, newCard)
}

// 发送角色标签卡片
func SendRoleTagsCard(ctx context.Context, sessionId *string, msgId *string, roleTags []string) {
	newCard, _ := newSendCard(
		withHeader("🛖 请选择角色类别", larkcard.TemplateIndigo),
		withRoleTagsBtn(sessionId, roleTags...),
		withNote("提醒：选择角色所属分类，以便我们为您推荐更多相关角色。"))
	replyCard(ctx, msgId, newCard)
}

// 发送角色列表卡片
func SendRoleListCard(ctx context.Context, sessionId *string, msgId *string, roleTag string, roleList []string) {
	newCard, _ := newSendCard(
		withHeader("🛖 角色列表"+" - "+roleTag, larkcard.TemplateIndigo),
		withRoleBtn(sessionId, roleList...),
		withNote("提醒：选择内置场景，快速进入角色扮演模式。"))
	replyCard(ctx, msgId, newCard)
}

// 发送处理中卡片
func sendOnProcessCard(ctx context.Context, sessionId *string, msgId *string) (*string, error) {
	content := "正在思考中，请稍等..."
	card, err := newSendCardWithOutHeader(withMainText(content))
	if err != nil {
		return nil, err
	}
	
	cardId, err := replyCardWithBackId(ctx, msgId, card)
	if err != nil {
		return nil, err
	}
	
	return cardId, nil
}
