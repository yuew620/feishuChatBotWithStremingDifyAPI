package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/initialization"
	"start-feishubot/services"
	"start-feishubot/services/cardcreator"
	"start-feishubot/services/cardservice"
	"start-feishubot/services/openai"
)


// 全局序列号计数器
var sequenceCounter int64

// 获取下一个序列号
func getNextSequence() int64 {
	return atomic.AddInt64(&sequenceCounter, 1)
}

// Token缓存相关变量
var (
	tokenCache     string
	tokenExpiry    time.Time
	tokenCacheMu   sync.RWMutex
)

// 获取tenant_access_token（带缓存）
func getTenantAccessToken(ctx context.Context) (string, error) {
	// 使用读锁检查缓存
	tokenCacheMu.RLock()
	if tokenCache != "" && time.Now().Before(tokenExpiry.Add(-5*time.Minute)) { // 提前5分钟刷新
		token := tokenCache
		tokenCacheMu.RUnlock()
		return token, nil
	}
	tokenCacheMu.RUnlock()
	
	// 使用写锁更新缓存
	tokenCacheMu.Lock()
	defer tokenCacheMu.Unlock()
	
	// 双重检查，避免多次刷新
	if tokenCache != "" && time.Now().Before(tokenExpiry.Add(-5*time.Minute)) {
		return tokenCache, nil
	}
	
	// 以下是原始获取token的逻辑
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
		// 如果获取失败但旧token仍有效，继续使用旧token
		if tokenCache != "" && time.Now().Before(tokenExpiry) {
			log.Printf("Failed to refresh token, using existing token: %v", err)
			return tokenCache, nil
		}
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// 如果获取失败但旧token仍有效，继续使用旧token
		if tokenCache != "" && time.Now().Before(tokenExpiry) {
			log.Printf("Failed to refresh token (status %d), using existing token: %s", resp.StatusCode, string(body))
			return tokenCache, nil
		}
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
		// 如果解析失败但旧token仍有效，继续使用旧token
		if tokenCache != "" && time.Now().Before(tokenExpiry) {
			log.Printf("Failed to decode token response, using existing token: %v", err)
			return tokenCache, nil
		}
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	if result.Code != 0 {
		// 如果API返回错误但旧token仍有效，继续使用旧token
		if tokenCache != "" && time.Now().Before(tokenExpiry) {
			log.Printf("API error when refreshing token, using existing token: code=%d, msg=%s", result.Code, result.Msg)
			return tokenCache, nil
		}
		return "", fmt.Errorf("API error: code=%d, msg=%s", result.Code, result.Msg)
	}
	
	// 更新缓存
	tokenCache = result.TenantAccessToken
	// 使用API返回的过期时间，默认减去5分钟作为安全边界
	expiresIn := result.Expire
	if expiresIn == 0 {
		expiresIn = 7200 // 默认2小时
	}
	tokenExpiry = time.Now().Add(time.Duration(expiresIn-300) * time.Second)
	
	return result.TenantAccessToken, nil
}


// 发送卡片实体
func sendCardEntity(ctx context.Context, cardID string, receiveID string) (string, error) {
	startTime := time.Now()
	log.Printf("[Timing] Starting to send card entity")
	
	// 获取tenant_access_token
	tokenStart := time.Now()
	token, err := getTenantAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get tenant_access_token: %w", err)
	}
	
	// 构建卡片实体内容
	cardContent := map[string]interface{}{
		"type": "card",
		"data": map[string]interface{}{
			"card_id": cardID,
		},
	}
	
	// 序列化卡片内容
	cardContentStr, err := json.Marshal(cardContent)
	if err != nil {
		return "", fmt.Errorf("failed to marshal card content: %w", err)
	}
	
	// 构建请求体
	reqBody := map[string]interface{}{
		"receive_id": receiveID,
		"msg_type": "interactive",
		"content": string(cardContentStr),
		"uuid": uuid.New().String(),
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// 构建请求URL
	url := "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id"
	
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// 发送请求
	requestStart := time.Now()
	log.Printf("[Timing] Token fetch took: %v ms", time.Since(tokenStart).Milliseconds())
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	log.Printf("[Timing] Card send API request took: %v ms", time.Since(requestStart).Milliseconds())
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	
	if result.Code != 0 {
		return "", fmt.Errorf("API error: code=%d, msg=%s", result.Code, result.Msg)
	}
	
	log.Printf("[Timing] Total card entity sending took: %v ms", time.Since(startTime).Milliseconds())
	return result.Data.MessageID, nil
}


// 流式更新文本内容
func streamUpdateText(ctx context.Context, cardId string, elementId string, content string) error {
	log.Printf("Attempting to update card: cardId=%s, elementId=%s, contentLength=%d", cardId, elementId, len(content))
	
	// 获取tenant_access_token
	token, err := getTenantAccessToken(ctx)
	if err != nil {
		log.Printf("Failed to get tenant_access_token: %v", err)
		return fmt.Errorf("failed to get tenant_access_token: %w", err)
	}
	
	// 获取序列号和UUID
	sequence := getNextSequence()
	reqUuid := uuid.New().String()
	
	// 构建请求体
	reqBody := map[string]interface{}{
		"content":  content,
		"sequence": sequence,
		"uuid":     reqUuid,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Failed to marshal request body: %v", err)
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// 构建请求URL
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/cardkit/v1/cards/%s/elements/%s/content", cardId, elementId)
	log.Printf("Making request to URL: %s", url)
	log.Printf("Request body: sequence=%d, uuid=%s, contentLength=%d", sequence, reqUuid, len(content))
	
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(jsonBody))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to send request: %v", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("API error: status=%d, body=%s", resp.StatusCode, string(body))
		return fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 读取并记录响应
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("Card update successful: status=%d, response=%s", resp.StatusCode, string(respBody))
	
	return nil
}

// 关闭流式更新模式
func closeStreamingMode(ctx context.Context, cardId string) error {
	// 获取tenant_access_token
	token, err := getTenantAccessToken(ctx)
	if err != nil {
		log.Printf("Warning: Failed to get token for closing streaming mode: %v", err)
		return nil // 不返回错误，因为这不是关键操作
	}
	
	// 构建请求体
	reqBody := map[string]interface{}{
		"config": map[string]interface{}{
			"streaming_mode": false,
		},
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Warning: Failed to marshal request body for closing streaming mode: %v", err)
		return nil // 不返回错误，因为这不是关键操作
	}

	// 构建请求URL
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/cardkit/v1/cards/%s/config", cardId)
	log.Printf("Making request to close streaming mode: %s", url)
	
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(jsonBody))
	if err != nil {
		log.Printf("Warning: Failed to create request for closing streaming mode: %v", err)
		return nil // 不返回错误，因为这不是关键操作
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Warning: Failed to send request for closing streaming mode: %v", err)
		return nil // 不返回错误，因为这不是关键操作
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Warning: Failed to close streaming mode: API error: status=%d, body=%s", resp.StatusCode, string(body))
		return nil // 不返回错误，因为这不是关键操作
	}
	
	log.Printf("Successfully closed streaming mode for card: %s", cardId)
	return nil
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
	
	// 创建基本元素
	mainElement := larkcard.NewMessageCardDiv().
		Fields([]*larkcard.MessageCardField{larkcard.NewMessageCardField().
			Text(larkcard.NewMessageCardPlainText().
				Content(msg).
				Build()).
			IsShort(false).
			Build()}).
		Build()
	
	// 获取JSON字符串
	elementJSON, err := json.Marshal(mainElement)
	if err != nil {
		return mainElement
	}
	
	// 解析JSON以添加element_id
	var elementMap map[string]interface{}
	if err := json.Unmarshal(elementJSON, &elementMap); err != nil {
		return mainElement
	}
	
	// 添加element_id
	elementMap["element_id"] = "content_block"
	
	// 重新序列化
	modifiedJSON, err := json.Marshal(elementMap)
	if err != nil {
		return mainElement
	}
	
	// 创建新的元素
	var newElement larkcard.MessageCardElement
	if err := json.Unmarshal(modifiedJSON, &newElement); err != nil {
		return mainElement
	}
	
	return newElement
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
	// 创建按钮代替下拉菜单
	// 由于SDK版本限制，我们使用按钮代替下拉菜单
	if len(options) > 0 {
		// 使用第一个选项创建按钮
		btn := larkcard.NewMessageCardEmbedButton().
			Type(larkcard.MessageCardButtonTypePrimary).
			Value(value).
			Text(larkcard.NewMessageCardPlainText().
				Content(placeHolder + ": " + options[0].label).
				Build())
		
		return btn
	}
	
	// 如果没有选项，创建一个默认按钮
	btn := larkcard.NewMessageCardEmbedButton().
		Type(larkcard.MessageCardButtonTypePrimary).
		Value(value).
		Text(larkcard.NewMessageCardPlainText().
			Content(placeHolder).
			Build())
	
	return btn
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
func updateTextCard(ctx context.Context, msg string, cardInfo *CardInfo) error {
	log.Printf("Updating card text: cardId=%s, elementId=%s, contentLength=%d", 
		cardInfo.CardEntityId, cardInfo.ElementId, len(msg))
	
	// 使用卡片实体ID和元素ID更新卡片内容
	err := streamUpdateText(ctx, cardInfo.CardEntityId, cardInfo.ElementId, msg)
	if err != nil {
		log.Printf("Error in updateTextCard: %v", err)
		return fmt.Errorf("failed to stream update text: %w", err)
	}
	
	log.Printf("Successfully updated card text: cardId=%s, elementId=%s", 
		cardInfo.CardEntityId, cardInfo.ElementId)
	return nil
}

// 更新最终卡片
func updateFinalCard(ctx context.Context, msg string, cardInfo *CardInfo) error {
	// 使用卡片实体ID和元素ID更新卡片内容
	err := streamUpdateText(ctx, cardInfo.CardEntityId, cardInfo.ElementId, msg)
	if err != nil {
		return fmt.Errorf("failed to update final card: %w", err)
	}
	
	// 可选：关闭流式更新模式
	err = closeStreamingMode(ctx, cardInfo.CardEntityId)
	if err != nil {
		log.Printf("Failed to close streaming mode: %v", err)
		// 不返回错误，因为这不是关键操作
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

// 创建简化的卡片JSON
func createSimpleCard(content string) (string, error) {
	// 使用结构体和标准JSON库，而不是字符串拼接
	cardStruct := struct {
		Schema string `json:"schema"`
		Config struct {
			StreamingMode bool `json:"streaming_mode"`
			UpdateMulti   bool `json:"update_multi"`
		} `json:"config"`
		Body struct {
			Elements []struct {
				Tag       string `json:"tag"`
				Content   string `json:"content"`
				ElementID string `json:"element_id"`
			} `json:"elements"`
		} `json:"body"`
	}{
		Schema: "2.0",
		Config: struct {
			StreamingMode bool `json:"streaming_mode"`
			UpdateMulti   bool `json:"update_multi"`
		}{
			StreamingMode: true,
			UpdateMulti:   true,
		},
		Body: struct {
			Elements []struct {
				Tag       string `json:"tag"`
				Content   string `json:"content"`
				ElementID string `json:"element_id"`
			} `json:"elements"`
		}{
			Elements: []struct {
				Tag       string `json:"tag"`
				Content   string `json:"content"`
				ElementID string `json:"element_id"`
			}{
				{
					Tag:       "markdown",
					Content:   content,
					ElementID: "content_block",
				},
			},
		},
	}
	
	// 使用标准库进行JSON序列化，处理转义和格式
	jsonBytes, err := json.Marshal(cardStruct)
	if err != nil {
		return "", fmt.Errorf("failed to marshal card: %w", err)
	}
	
	return string(jsonBytes), nil
}


// 发送处理中卡片并并行处理Dify消息
func sendOnProcessCardAndDify(ctx context.Context, sessionId *string, msgId *string, difyHandler func(context.Context) error) (*CardInfo, error) {
	// 创建一个错误通道用于收集错误
	errChan := make(chan error, 2)
	var cardInfo *CardInfo
	var cardInfoMu sync.Mutex
	
	// 启动发送卡片的goroutine
	go func() {
		info, err := sendOnProcessCard(ctx, sessionId, msgId)
		if err != nil {
			errChan <- fmt.Errorf("发送卡片失败: %w", err)
			return
		}
		cardInfoMu.Lock()
		cardInfo = info
		cardInfoMu.Unlock()
		errChan <- nil
	}()
	
	// 启动处理Dify消息的goroutine
	go func() {
		if err := difyHandler(ctx); err != nil {
			errChan <- fmt.Errorf("处理Dify消息失败: %w", err)
			return
		}
		errChan <- nil
	}()
	
	// 等待两个goroutine完成
	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			log.Printf("Error in parallel processing: %v", err)
			// 继续等待另一个goroutine，但记录错误
			continue
		}
	}
	
	// 如果卡片发送成功，返回卡片信息
	cardInfoMu.Lock()
	defer cardInfoMu.Unlock()
	if cardInfo != nil {
		return cardInfo, nil
	}
	
	// 如果卡片发送失败，使用回退方法
	return sendOnProcessCardFallback(ctx, sessionId, msgId)
}

// 发送处理中卡片
func sendOnProcessCard(ctx context.Context, sessionId *string, msgId *string) (*CardInfo, error) {
	log.Printf("Sending processing card for message ID: %s", *msgId)
	
	// 获取卡片池实例
	cardPool := cardservice.GetCardPool()
	if cardPool == nil {
		log.Printf("Card pool not initialized, falling back to direct creation")
		return sendOnProcessCardFallback(ctx, sessionId, msgId)
	}
	
	// 持续尝试发送卡片，直到成功或完全失败
	for {
		// 从卡片池获取卡片ID（会自动触发异步创建新卡片）
		cardEntityId, err := cardPool.GetCard(ctx)
		if err != nil {
			log.Printf("Failed to get card from pool: %v, falling back to direct creation", err)
			return sendOnProcessCardFallback(ctx, sessionId, msgId)
		}
		
		// 直接使用回复方式发送卡片，不需要验证卡片有效性
		client := initialization.GetLarkClient()
		resp, err := client.Im.Message.Reply(ctx, larkim.NewReplyMessageReqBuilder().
			MessageId(*msgId).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType(larkim.MsgTypeInteractive).
				Uuid(uuid.New().String()).
				Content(fmt.Sprintf("{\"type\":\"card\",\"data\":{\"card_id\":\"%s\"}}", cardEntityId)).
				Build()).
			Build())
		
		// 如果发送失败，说明卡片可能已过期，继续获取新卡片重试
		if err != nil || !resp.Success() {
			if err != nil {
				log.Printf("Failed to send card: %v, trying next card", err)
			} else {
				log.Printf("API error: code=%d, msg=%s, trying next card", resp.Code, resp.Msg)
			}
			continue
		}
		
		// 发送成功
		messageId := *resp.Data.MessageId
		log.Printf("Successfully sent card entity using reply method, message ID: %s", messageId)
		
		return &CardInfo{
			CardEntityId: cardEntityId,
			MessageId:    messageId,
			ElementId:    "content_block",
		}, nil
	}
}

// 回退方法，使用SDK直接回复消息
func sendOnProcessCardFallback(ctx context.Context, sessionId *string, msgId *string) (*CardInfo, error) {
	log.Printf("Using fallback method for sending processing card")
	
	// 创建一个简单的卡片内容
	content := "正在思考中，请稍等..."
	
	// 创建符合飞书卡片规范的JSON
	cardJSON := map[string]interface{}{
		"schema": "2.0",
		"config": map[string]interface{}{
			"streaming_mode": true,
			"summary": map[string]interface{}{
				"content": "[生成中]",
			},
		},
		"body": map[string]interface{}{
			"elements": []map[string]interface{}{
				{
					"tag": "markdown",
					"content": content,
					"element_id": "content_block",
				},
				{
					"tag": "note",
					"elements": []map[string]interface{}{
						{
							"tag": "plain_text",
							"content": "正在处理中，请稍等...",
						},
					},
				},
			},
		},
	}
	
	// 序列化卡片JSON
	cardJSONStr, err := json.Marshal(cardJSON)
	if err != nil {
		log.Printf("Failed to marshal card JSON: %v", err)
		return nil, fmt.Errorf("failed to marshal card JSON: %w", err)
	}
	
	// 使用SDK回复消息
	client := initialization.GetLarkClient()
	resp, err := client.Im.Message.Reply(ctx, larkim.NewReplyMessageReqBuilder().
		MessageId(*msgId).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			Uuid(uuid.New().String()).
			Content(string(cardJSONStr)).
			Build()).
		Build())
	
	if err != nil {
		log.Printf("Failed to reply with card: %v", err)
		return nil, fmt.Errorf("failed to reply with card: %w", err)
	}
	
	if !resp.Success() {
		log.Printf("API error: code=%d, msg=%s", resp.Code, resp.Msg)
		return nil, fmt.Errorf("API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	
	messageId := resp.Data.MessageId
	log.Printf("Successfully sent card using fallback method, message ID: %s", *messageId)
	
	// 在回退方法中，我们使用消息ID作为卡片ID
	return &CardInfo{
		CardEntityId: *messageId,
		MessageId:    *messageId,
		ElementId:    "content_block",
	}, nil
}

// 这些函数已在 common.go 中定义，不需要重复定义
