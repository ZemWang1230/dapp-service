package email

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/smtp"
	"strconv"

	"timelocker-backend/internal/config"
	"timelocker-backend/pkg/logger"
)

// Service 邮件服务接口
type Service interface {
	SendVerificationCode(to, code string) error
	SendTimelockNotification(to, timelockAddress, eventType string, transactionHash *string, isEmergency bool, replyToken *string) error
	GenerateVerificationCode() string
	GenerateReplyToken() string
}

type service struct {
	config *config.EmailConfig
}

// NewService 创建邮件服务
func NewService(config *config.EmailConfig) Service {
	return &service{
		config: config,
	}
}

// EmailData 邮件模板数据
type EmailData struct {
	To               string
	Code             string
	TimelockAddress  string
	EventType        string
	TransactionHash  string
	IsEmergency      bool
	ReplyToken       string
	BaseURL          string
	ReplyURL         string
	EventTitle       string
	EventDescription string
}

// GenerateVerificationCode 生成6位数字验证码
func (s *service) GenerateVerificationCode() string {
	// 生成6位随机数字
	bytes := make([]byte, 3)
	rand.Read(bytes)
	code := ""
	for _, b := range bytes {
		code += fmt.Sprintf("%02d", int(b)%100)
	}
	if len(code) > 6 {
		code = code[:6]
	}
	// 如果不足6位，补充随机数字
	for len(code) < 6 {
		bytes := make([]byte, 1)
		rand.Read(bytes)
		code += strconv.Itoa(int(bytes[0]) % 10)
	}
	return code
}

// GenerateReplyToken 生成应急邮件回复token
func (s *service) GenerateReplyToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// SendVerificationCode 发送验证码邮件
func (s *service) SendVerificationCode(to, code string) error {
	subject := "TimeLocker - Email Verification Code"

	// 验证码邮件模板
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Email Verification</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #4CAF50; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background: #f9f9f9; }
        .code { font-size: 24px; font-weight: bold; color: #4CAF50; text-align: center; padding: 20px; background: white; margin: 20px 0; border: 2px dashed #4CAF50; }
        .footer { padding: 20px; text-align: center; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>TimeLocker Email Verification</h1>
        </div>
        <div class="content">
            <p>Dear User,</p>
            <p>Thank you for adding your email address to TimeLocker notification system. To complete the setup, please use the verification code below:</p>
            <div class="code">{{.Code}}</div>
            <p>This code will expire in 10 minutes.</p>
            <p>If you didn't request this verification, please ignore this email.</p>
        </div>
        <div class="footer">
            <p>This is an automated message from TimeLocker. Please do not reply to this email.</p>
        </div>
    </div>
</body>
</html>`

	data := EmailData{
		To:   to,
		Code: code,
	}

	return s.sendEmail(to, subject, tmpl, data)
}

// SendTimelockNotification 发送timelock通知邮件
func (s *service) SendTimelockNotification(to, timelockAddress, eventType string, transactionHash *string, isEmergency bool, replyToken *string) error {
	// 根据事件类型设置标题和描述
	eventTitle, eventDescription := s.getEventInfo(eventType)

	subject := fmt.Sprintf("TimeLocker Alert - %s", eventTitle)

	// 构建邮件数据
	data := EmailData{
		To:               to,
		TimelockAddress:  timelockAddress,
		EventType:        eventType,
		EventTitle:       eventTitle,
		EventDescription: eventDescription,
		IsEmergency:      isEmergency,
		BaseURL:          s.config.BaseURL,
	}

	if transactionHash != nil {
		data.TransactionHash = *transactionHash
	}

	if replyToken != nil {
		data.ReplyToken = *replyToken
		data.ReplyURL = fmt.Sprintf("%s/api/v1/email-notifications/emergency-reply?token=%s", s.config.BaseURL, *replyToken)
	}

	var tmpl string
	if isEmergency {
		tmpl = s.getEmergencyEmailTemplate()
	} else {
		tmpl = s.getNormalEmailTemplate()
	}

	return s.sendEmail(to, subject, tmpl, data)
}

// getEventInfo 获取事件信息
func (s *service) getEventInfo(eventType string) (string, string) {
	switch eventType {
	case "proposal_created":
		return "New Proposal Created", "A new transaction proposal has been queued in your monitored timelock contract."
	case "proposal_canceled":
		return "Proposal Canceled", "A transaction proposal has been canceled in your monitored timelock contract."
	case "ready_to_execute":
		return "Ready to Execute", "A transaction proposal is now ready to be executed in your monitored timelock contract."
	case "executed":
		return "Transaction Executed", "A transaction has been successfully executed in your monitored timelock contract."
	case "expired":
		return "Transaction Expired", "A transaction proposal has expired in your monitored timelock contract."
	default:
		return "TimeLocker Notification", "An event has occurred in your monitored timelock contract."
	}
}

// getNormalEmailTemplate 获取普通邮件模板
func (s *service) getNormalEmailTemplate() string {
	return `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>TimeLocker Notification</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #2196F3; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background: #f9f9f9; }
        .info-box { background: white; padding: 15px; margin: 15px 0; border-left: 4px solid #2196F3; }
        .label { font-weight: bold; color: #666; }
        .value { color: #333; word-break: break-all; }
        .footer { padding: 20px; text-align: center; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.EventTitle}}</h1>
        </div>
        <div class="content">
            <p>Dear User,</p>
            <p>{{.EventDescription}}</p>
            
            <div class="info-box">
                <div class="label">Event Type:</div>
                <div class="value">{{.EventType}}</div>
            </div>
            
            <div class="info-box">
                <div class="label">Timelock Contract:</div>
                <div class="value">{{.TimelockAddress}}</div>
            </div>
            
            {{if .TransactionHash}}
            <div class="info-box">
                <div class="label">Transaction Hash:</div>
                <div class="value">{{.TransactionHash}}</div>
            </div>
            {{end}}
            
            <div class="info-box">
                <div class="label">Time:</div>
                <div class="value">{{.Timestamp}}</div>
            </div>
            
            <p>Please check your TimeLocker dashboard for more details.</p>
        </div>
        <div class="footer">
            <p>This is an automated notification from TimeLocker. Please do not reply to this email.</p>
        </div>
    </div>
</body>
</html>`
}

// getEmergencyEmailTemplate 获取应急邮件模板
func (s *service) getEmergencyEmailTemplate() string {
	return `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>TimeLocker Emergency Alert</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #f44336; color: white; padding: 20px; text-align: center; }
        .emergency-badge { background: #ff9800; color: white; padding: 5px 15px; border-radius: 20px; font-weight: bold; }
        .content { padding: 20px; background: #f9f9f9; }
        .info-box { background: white; padding: 15px; margin: 15px 0; border-left: 4px solid #f44336; }
        .label { font-weight: bold; color: #666; }
        .value { color: #333; word-break: break-all; }
        .reply-button { 
            display: inline-block; 
            background: #4CAF50; 
            color: white; 
            padding: 15px 30px; 
            text-decoration: none; 
            border-radius: 5px; 
            font-weight: bold;
            text-align: center;
            margin: 20px 0;
        }
        .warning { background: #fff3cd; border: 1px solid #ffeaa7; padding: 15px; margin: 15px 0; border-radius: 5px; }
        .footer { padding: 20px; text-align: center; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <span class="emergency-badge">🚨 EMERGENCY ALERT</span>
            <h1>{{.EventTitle}}</h1>
        </div>
        <div class="content">
            <div class="warning">
                <strong>⚠️ This is an emergency notification requiring your immediate attention!</strong>
            </div>
            
            <p>Dear User,</p>
            <p>{{.EventDescription}}</p>
            
            <div class="info-box">
                <div class="label">Event Type:</div>
                <div class="value">{{.EventType}}</div>
            </div>
            
            <div class="info-box">
                <div class="label">Timelock Contract:</div>
                <div class="value">{{.TimelockAddress}}</div>
            </div>
            
            {{if .TransactionHash}}
            <div class="info-box">
                <div class="label">Transaction Hash:</div>
                <div class="value">{{.TransactionHash}}</div>
            </div>
            {{end}}
            
            <div class="info-box">
                <div class="label">Time:</div>
                <div class="value">{{.Timestamp}}</div>
            </div>
            
            <div style="text-align: center; margin: 30px 0;">
                <a href="{{.ReplyURL}}" class="reply-button">✅ CONFIRM RECEIPT</a>
            </div>
            
            <div class="warning">
                <strong>Important:</strong> Please click the "CONFIRM RECEIPT" button above to acknowledge this notification. 
                If we don't receive confirmation from monitored emails within 2 hours, this alert will be resent automatically.
            </div>
            
            <p>Please check your TimeLocker dashboard immediately for more details.</p>
        </div>
        <div class="footer">
            <p>This is an emergency notification from TimeLocker. Please do not reply to this email directly.</p>
            <p>Use the confirmation button above to acknowledge receipt.</p>
        </div>
    </div>
</body>
</html>`
}

// sendEmail 发送邮件
func (s *service) sendEmail(to, subject, tmpl string, data EmailData) error {
	// 解析模板
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		logger.Error("SendEmail Parse Template Error: ", err, "to", to)
		return fmt.Errorf("failed to parse email template: %w", err)
	}

	// 渲染模板
	var body bytes.Buffer
	if err := t.Execute(&body, data); err != nil {
		logger.Error("SendEmail Execute Template Error: ", err, "to", to)
		return fmt.Errorf("failed to execute email template: %w", err)
	}

	// 构建邮件
	message := s.buildMessage(to, subject, body.String())

	// 发送邮件
	err = s.sendSMTP(to, message)
	if err != nil {
		logger.Error("SendEmail SMTP Error: ", err, "to", to, "subject", subject)
		return err
	}

	logger.Info("SendEmail: ", "to", to, "subject", subject)
	return nil
}

// buildMessage 构建邮件消息
func (s *service) buildMessage(to, subject, body string) string {
	return fmt.Sprintf("From: %s <%s>\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=\"UTF-8\"\r\n"+
		"\r\n%s",
		s.config.FromName, s.config.FromEmail, to, subject, body)
}

// sendSMTP 通过SMTP发送邮件
func (s *service) sendSMTP(to, message string) error {
	// SMTP服务器地址
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)

	// 认证信息
	auth := smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)

	// TLS配置
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         s.config.SMTPHost,
	}

	// 连接到SMTP服务器
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// 创建SMTP客户端
	client, err := smtp.NewClient(conn, s.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	// 认证
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// 设置发件人
	if err := client.Mail(s.config.FromEmail); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// 设置收件人
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// 发送邮件内容
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	defer writer.Close()

	if _, err := writer.Write([]byte(message)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}
