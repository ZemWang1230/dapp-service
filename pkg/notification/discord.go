package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DiscordSender Discord消息发送器
type DiscordSender struct{}

// NewDiscordSender 创建Discord发送器实例
func NewDiscordSender() *DiscordSender {
	return &DiscordSender{}
}

// DiscordMessage Discord消息结构
type DiscordMessage struct {
	Content string `json:"content"`
}

// SendMessage 发送Discord消息
func (s *DiscordSender) SendMessage(webhookURL, message string) error {
	discordMsg := DiscordMessage{
		Content: message,
	}

	jsonData, err := json.Marshal(discordMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal discord message: %w", err)
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送请求
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send discord message: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}
