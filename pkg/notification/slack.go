package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackSender Slack消息发送器
type SlackSender struct{}

// NewSlackSender 创建Slack发送器实例
func NewSlackSender() *SlackSender {
	return &SlackSender{}
}

// SlackMessage Slack消息结构
type SlackMessage struct {
	Text string `json:"text"`
}

// SendMessage 发送Slack消息
func (s *SlackSender) SendMessage(webhookURL, message string) error {
	slackMsg := SlackMessage{
		Text: message,
	}

	jsonData, err := json.Marshal(slackMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送请求
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}
