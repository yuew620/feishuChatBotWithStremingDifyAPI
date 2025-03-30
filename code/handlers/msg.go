package handlers

import (
	"encoding/json"
	"fmt"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"start-feishubot/services/config"
)

// GetTextContent extracts text content from message
func GetTextContent(msg *larkim.Message) (string, error) {
	var textContent struct {
		Text string `json:"text"`
	}
	err := json.Unmarshal([]byte(*msg.Content), &textContent)
	if err != nil {
		return "", err
	}
	return textContent.Text, nil
}

// GetImageContent extracts image key from message
func GetImageContent(msg *larkim.Message) (string, error) {
	var imageContent struct {
		ImageKey string `json:"image_key"`
	}
	err := json.Unmarshal([]byte(*msg.Content), &imageContent)
	if err != nil {
		return "", err
	}
	return imageContent.ImageKey, nil
}

// GetAudioContent extracts audio key from message
func GetAudioContent(msg *larkim.Message) (string, error) {
	var audioContent struct {
		FileKey string `json:"file_key"`
	}
	err := json.Unmarshal([]byte(*msg.Content), &audioContent)
	if err != nil {
		return "", err
	}
	return audioContent.FileKey, nil
}

// GetFileContent extracts file key from message
func GetFileContent(msg *larkim.Message) (string, error) {
	var fileContent struct {
		FileKey string `json:"file_key"`
	}
	err := json.Unmarshal([]byte(*msg.Content), &fileContent)
	if err != nil {
		return "", err
	}
	return fileContent.FileKey, nil
}

// GetMentionedMessage extracts mentioned message from event
func GetMentionedMessage(event *larkim.P2MessageReceiveV1, cfg config.Config) bool {
	mentions := event.Event.Message.Mentions
	if len(mentions) != 1 {
		return false
	}
	return *mentions[0].Name == cfg.GetFeishuAppID()
}

// GetMessageType extracts message type from event
func GetMessageType(event *larkim.P2MessageReceiveV1) string {
	return *event.Event.Message.MessageType
}

// GetChatType extracts chat type from event
func GetChatType(event *larkim.P2MessageReceiveV1) string {
	return *event.Event.Message.ChatType
}

// GetContent extracts content from event
func GetContent(event *larkim.P2MessageReceiveV1) (*larkim.Message, error) {
	content := event.Event.Message
	if content == nil {
		return nil, fmt.Errorf("empty message content")
	}
	return content, nil
}
