package email_notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/email_notification"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/email"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

var (
	ErrEmailAlreadyExists      = errors.New("email already exists for this wallet")
	ErrEmailNotFound           = errors.New("email notification not found")
	ErrEmailNotVerified        = errors.New("email not verified")
	ErrInvalidVerificationCode = errors.New("invalid or expired verification code")
	ErrTimelockNotBelongToUser = errors.New("timelock contract does not belong to user")
	ErrEmergencyReplyNotFound  = errors.New("emergency reply not found")
	ErrEmergencyAlreadyReplied = errors.New("emergency email already replied")
)

// Service 邮件通知服务接口
type Service interface {
	// 邮件通知配置管理
	AddEmailNotification(ctx context.Context, walletAddress string, req *types.AddEmailNotificationRequest) (*types.EmailNotificationResponse, error)
	VerifyEmail(ctx context.Context, walletAddress string, req *types.VerifyEmailRequest) error
	UpdateEmailNotification(ctx context.Context, walletAddress string, email string, req *types.UpdateEmailNotificationRequest) (*types.EmailNotificationResponse, error)
	DeleteEmailNotification(ctx context.Context, walletAddress string, email string) error
	GetEmailNotifications(ctx context.Context, walletAddress string, page, pageSize int) (*types.EmailNotificationListResponse, error)
	GetEmailNotification(ctx context.Context, walletAddress string, email string) (*types.EmailNotificationResponse, error)
	ResendVerificationCode(ctx context.Context, walletAddress string, req *types.ResendVerificationRequest) error

	// 邮件发送相关
	SendTimelockNotification(ctx context.Context, timelockAddress, eventType string, transactionHash *string) error
	GetEmailSendLogs(ctx context.Context, walletAddress string, page, pageSize int) ([]types.EmailSendLogResponse, int, error)

	// 应急邮件回复
	ReplyEmergencyEmail(ctx context.Context, token string) (*types.EmergencyReplyResponse, error)

	// 内部方法，用于timelock合约选择监听邮箱
	GetVerifiedEmailsByWallet(ctx context.Context, walletAddress string) ([]types.EmailNotificationResponse, error)
	UpdateTimelockContracts(ctx context.Context, walletAddress string, email string, timelockContracts []string) error
}

type service struct {
	emailRepo    email_notification.Repository
	emailService email.Service
	config       *config.EmailConfig
}

// NewService 创建邮件通知服务
func NewService(emailRepo email_notification.Repository, emailService email.Service, config *config.EmailConfig) Service {
	return &service{
		emailRepo:    emailRepo,
		emailService: emailService,
		config:       config,
	}
}

// AddEmailNotification 添加邮件通知配置
func (s *service) AddEmailNotification(ctx context.Context, walletAddress string, req *types.AddEmailNotificationRequest) (*types.EmailNotificationResponse, error) {
	// 检查邮箱是否已存在
	_, err := s.emailRepo.GetEmailNotificationByWalletAndEmail(ctx, walletAddress, req.Email)
	if err == nil {
		logger.Error("AddEmailNotification Error: ", ErrEmailAlreadyExists, "wallet_address", walletAddress, "email", req.Email)
		return nil, ErrEmailAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("AddEmailNotification Database Error: ", err, "wallet_address", walletAddress, "email", req.Email)
		return nil, fmt.Errorf("database error: %w", err)
	}

	// 将timelock合约列表转换为JSON
	timelockContractsJSON, err := json.Marshal(req.TimelockContracts)
	if err != nil {
		logger.Error("AddEmailNotification JSON Marshal Error: ", err, "wallet_address", walletAddress, "email", req.Email)
		return nil, fmt.Errorf("failed to marshal timelock contracts: %w", err)
	}

	// 创建邮件通知配置
	notification := &types.EmailNotification{
		WalletAddress:     walletAddress,
		Email:             req.Email,
		EmailRemark:       req.EmailRemark,
		TimelockContracts: string(timelockContractsJSON),
		IsVerified:        false,
		IsActive:          true,
	}

	// 生成验证码
	code := s.emailService.GenerateVerificationCode()
	expiresAt := time.Now().Add(s.config.VerificationCodeExpiry)

	// 保存到数据库
	if err := s.emailRepo.CreateEmailNotification(ctx, notification); err != nil {
		logger.Error("AddEmailNotification Create Error: ", err, "wallet_address", walletAddress, "email", req.Email)
		return nil, fmt.Errorf("failed to create email notification: %w", err)
	}

	// 设置验证码
	if err := s.emailRepo.SetVerificationCode(ctx, walletAddress, req.Email, code, expiresAt); err != nil {
		logger.Error("AddEmailNotification SetVerificationCode Error: ", err, "wallet_address", walletAddress, "email", req.Email)
		return nil, fmt.Errorf("failed to set verification code: %w", err)
	}

	// 发送验证码邮件
	if err := s.emailService.SendVerificationCode(req.Email, code); err != nil {
		logger.Error("AddEmailNotification SendVerificationCode Error: ", err, "wallet_address", walletAddress, "email", req.Email)
		// 不返回错误，允许用户重新发送验证码
	}

	logger.Info("AddEmailNotification: ", "id", notification.ID, "wallet_address", walletAddress, "email", req.Email)

	// 转换为响应格式
	var timelockContracts []string
	json.Unmarshal([]byte(notification.TimelockContracts), &timelockContracts)

	return &types.EmailNotificationResponse{
		ID:                notification.ID,
		Email:             notification.Email,
		EmailRemark:       notification.EmailRemark,
		TimelockContracts: timelockContracts,
		IsVerified:        notification.IsVerified,
		IsActive:          notification.IsActive,
		CreatedAt:         notification.CreatedAt,
		UpdatedAt:         notification.UpdatedAt,
	}, nil
}

// VerifyEmail 验证邮箱
func (s *service) VerifyEmail(ctx context.Context, walletAddress string, req *types.VerifyEmailRequest) error {
	// 验证验证码
	err := s.emailRepo.VerifyEmailCode(ctx, walletAddress, req.Email, req.VerificationCode)
	if err != nil {
		logger.Error("VerifyEmail Error: ", err, "wallet_address", walletAddress, "email", req.Email)
		if err.Error() == "invalid or expired verification code" {
			return ErrInvalidVerificationCode
		}
		return fmt.Errorf("failed to verify email code: %w", err)
	}

	logger.Info("VerifyEmail: ", "wallet_address", walletAddress, "email", req.Email)
	return nil
}

// UpdateEmailNotification 更新邮件通知配置
func (s *service) UpdateEmailNotification(ctx context.Context, walletAddress string, email string, req *types.UpdateEmailNotificationRequest) (*types.EmailNotificationResponse, error) {
	// 获取现有配置
	notification, err := s.emailRepo.GetEmailNotificationByWalletAndEmail(ctx, walletAddress, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEmailNotFound
		}
		logger.Error("UpdateEmailNotification Get Error: ", err, "wallet_address", walletAddress, "email", email)
		return nil, fmt.Errorf("failed to get email notification: %w", err)
	}

	// 检查是否已验证
	if !notification.IsVerified {
		logger.Error("UpdateEmailNotification Error: ", ErrEmailNotVerified, "wallet_address", walletAddress, "email", email)
		return nil, ErrEmailNotVerified
	}

	// 更新字段
	notification.EmailRemark = req.EmailRemark

	// 将timelock合约列表转换为JSON
	timelockContractsJSON, err := json.Marshal(req.TimelockContracts)
	if err != nil {
		logger.Error("UpdateEmailNotification JSON Marshal Error: ", err, "wallet_address", walletAddress, "email", email)
		return nil, fmt.Errorf("failed to marshal timelock contracts: %w", err)
	}
	notification.TimelockContracts = string(timelockContractsJSON)

	// 保存更新
	if err := s.emailRepo.UpdateEmailNotification(ctx, notification); err != nil {
		logger.Error("UpdateEmailNotification Update Error: ", err, "wallet_address", walletAddress, "email", email)
		return nil, fmt.Errorf("failed to update email notification: %w", err)
	}

	logger.Info("UpdateEmailNotification: ", "id", notification.ID, "wallet_address", walletAddress, "email", email)

	// 转换为响应格式
	var timelockContracts []string
	json.Unmarshal([]byte(notification.TimelockContracts), &timelockContracts)

	return &types.EmailNotificationResponse{
		ID:                notification.ID,
		Email:             notification.Email,
		EmailRemark:       notification.EmailRemark,
		TimelockContracts: timelockContracts,
		IsVerified:        notification.IsVerified,
		IsActive:          notification.IsActive,
		CreatedAt:         notification.CreatedAt,
		UpdatedAt:         notification.UpdatedAt,
	}, nil
}

// DeleteEmailNotification 删除邮件通知配置
func (s *service) DeleteEmailNotification(ctx context.Context, walletAddress string, email string) error {
	// 获取配置
	notification, err := s.emailRepo.GetEmailNotificationByWalletAndEmail(ctx, walletAddress, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEmailNotFound
		}
		logger.Error("DeleteEmailNotification Get Error: ", err, "wallet_address", walletAddress, "email", email)
		return fmt.Errorf("failed to get email notification: %w", err)
	}

	// 删除配置
	if err := s.emailRepo.DeleteEmailNotification(ctx, notification.ID); err != nil {
		logger.Error("DeleteEmailNotification Delete Error: ", err, "id", notification.ID)
		return fmt.Errorf("failed to delete email notification: %w", err)
	}

	logger.Info("DeleteEmailNotification: ", "id", notification.ID, "wallet_address", walletAddress, "email", email)
	return nil
}

// GetEmailNotifications 获取邮件通知配置列表
func (s *service) GetEmailNotifications(ctx context.Context, walletAddress string, page, pageSize int) (*types.EmailNotificationListResponse, error) {
	// 参数验证
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 获取数据
	notifications, total, err := s.emailRepo.GetEmailNotificationsByWallet(ctx, walletAddress, page, pageSize)
	if err != nil {
		logger.Error("GetEmailNotifications Error: ", err, "wallet_address", walletAddress)
		return nil, fmt.Errorf("failed to get email notifications: %w", err)
	}

	// 转换为响应格式
	items := make([]types.EmailNotificationResponse, len(notifications))
	for i, notification := range notifications {
		var timelockContracts []string
		json.Unmarshal([]byte(notification.TimelockContracts), &timelockContracts)

		items[i] = types.EmailNotificationResponse{
			ID:                notification.ID,
			Email:             notification.Email,
			EmailRemark:       notification.EmailRemark,
			TimelockContracts: timelockContracts,
			IsVerified:        notification.IsVerified,
			IsActive:          notification.IsActive,
			CreatedAt:         notification.CreatedAt,
			UpdatedAt:         notification.UpdatedAt,
		}
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	logger.Info("GetEmailNotifications: ", "wallet_address", walletAddress, "total", total, "page", page)
	return &types.EmailNotificationListResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetEmailNotification 获取单个邮件通知配置
func (s *service) GetEmailNotification(ctx context.Context, walletAddress string, email string) (*types.EmailNotificationResponse, error) {
	notification, err := s.emailRepo.GetEmailNotificationByWalletAndEmail(ctx, walletAddress, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEmailNotFound
		}
		logger.Error("GetEmailNotification Error: ", err, "wallet_address", walletAddress, "email", email)
		return nil, fmt.Errorf("failed to get email notification: %w", err)
	}

	// 转换为响应格式
	var timelockContracts []string
	json.Unmarshal([]byte(notification.TimelockContracts), &timelockContracts)

	logger.Info("GetEmailNotification: ", "id", notification.ID, "wallet_address", walletAddress, "email", email)
	return &types.EmailNotificationResponse{
		ID:                notification.ID,
		Email:             notification.Email,
		EmailRemark:       notification.EmailRemark,
		TimelockContracts: timelockContracts,
		IsVerified:        notification.IsVerified,
		IsActive:          notification.IsActive,
		CreatedAt:         notification.CreatedAt,
		UpdatedAt:         notification.UpdatedAt,
	}, nil
}

// ResendVerificationCode 重发验证码
func (s *service) ResendVerificationCode(ctx context.Context, walletAddress string, req *types.ResendVerificationRequest) error {
	// 获取邮件配置
	notification, err := s.emailRepo.GetEmailNotificationByWalletAndEmail(ctx, walletAddress, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEmailNotFound
		}
		logger.Error("ResendVerificationCode Get Error: ", err, "wallet_address", walletAddress, "email", req.Email)
		return fmt.Errorf("failed to get email notification: %w", err)
	}

	// 检查是否已验证
	if notification.IsVerified {
		logger.Error("ResendVerificationCode Error: ", fmt.Errorf("email already verified"), "wallet_address", walletAddress, "email", req.Email)
		return fmt.Errorf("email already verified")
	}

	// 生成新验证码
	code := s.emailService.GenerateVerificationCode()
	expiresAt := time.Now().Add(s.config.VerificationCodeExpiry)

	// 更新验证码
	if err := s.emailRepo.SetVerificationCode(ctx, walletAddress, req.Email, code, expiresAt); err != nil {
		logger.Error("ResendVerificationCode SetVerificationCode Error: ", err, "wallet_address", walletAddress, "email", req.Email)
		return fmt.Errorf("failed to set verification code: %w", err)
	}

	// 发送验证码邮件
	if err := s.emailService.SendVerificationCode(req.Email, code); err != nil {
		logger.Error("ResendVerificationCode SendVerificationCode Error: ", err, "wallet_address", walletAddress, "email", req.Email)
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	logger.Info("ResendVerificationCode: ", "wallet_address", walletAddress, "email", req.Email)
	return nil
}

// SendTimelockNotification 发送timelock通知
func (s *service) SendTimelockNotification(ctx context.Context, timelockAddress, eventType string, transactionHash *string) error {
	// 获取监听该timelock合约的已验证邮箱
	notifications, err := s.emailRepo.GetVerifiedEmailsByTimelockContract(ctx, timelockAddress)
	if err != nil {
		logger.Error("SendTimelockNotification GetVerifiedEmails Error: ", err, "timelock_address", timelockAddress)
		return fmt.Errorf("failed to get verified emails: %w", err)
	}

	if len(notifications) == 0 {
		logger.Info("SendTimelockNotification: no verified emails found", "timelock_address", timelockAddress)
		return nil
	}

	// 检查该timelock合约是否启用应急模式
	isEmergency, err := s.checkTimelockEmergencyMode(ctx, timelockAddress)
	if err != nil {
		logger.Error("SendTimelockNotification CheckEmergencyMode Error: ", err, "timelock_address", timelockAddress)
		// 如果检查失败，使用全局配置作为后备
		isEmergency = s.config.EnableEmergencyMode
	}

	// 如果是应急模式且有交易hash，创建应急通知记录
	if isEmergency && transactionHash != nil {
		emergencyNotification := &types.EmergencyNotification{
			TimelockAddress: timelockAddress,
			TransactionHash: *transactionHash,
			EventType:       eventType,
			RepliedEmails:   0,
			IsCompleted:     false,
			NextSendAt:      &[]time.Time{time.Now().Add(s.config.EmergencyResendInterval)}[0],
			SendCount:       1,
		}

		// 创建应急通知记录，如果失败则降级为普通邮件
		if err := s.emailRepo.CreateEmergencyNotification(ctx, emergencyNotification); err != nil {
			logger.Warn("SendTimelockNotification: failed to create emergency notification, falling back to normal mode", "error", err)
			isEmergency = false
		}
	}

	// 为每个邮箱发送通知
	for _, notification := range notifications {
		// 检查该邮箱是否真的监听这个合约
		var timelockContracts []string
		if err := json.Unmarshal([]byte(notification.TimelockContracts), &timelockContracts); err != nil {
			logger.Error("SendTimelockNotification JSON Unmarshal Error: ", err, "email", notification.Email)
			continue
		}

		// 检查timelock合约是否在监听列表中
		found := false
		for _, contract := range timelockContracts {
			if contract == timelockAddress {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		// 生成应急回复token
		var replyToken *string
		if isEmergency {
			token := s.emailService.GenerateReplyToken()
			replyToken = &token
		}

		// 创建邮件发送记录
		sendLog := &types.EmailSendLog{
			EmailNotificationID: notification.ID,
			Email:               notification.Email,
			TimelockAddress:     timelockAddress,
			TransactionHash:     transactionHash,
			EventType:           eventType,
			Subject:             s.generateEmailSubject(eventType),
			Content:             s.generateEmailContent(eventType, timelockAddress, transactionHash),
			IsEmergency:         isEmergency,
			EmergencyReplyToken: replyToken,
			IsReplied:           false,
			SendStatus:          types.SendStatusPending,
			SendAttempts:        0,
		}

		if err := s.emailRepo.CreateEmailSendLog(ctx, sendLog); err != nil {
			logger.Error("SendTimelockNotification CreateEmailSendLog Error: ", err, "email", notification.Email)
			continue
		}

		// 发送邮件
		if err := s.emailService.SendTimelockNotification(
			notification.Email,
			timelockAddress,
			eventType,
			transactionHash,
			isEmergency,
			replyToken,
		); err != nil {
			// 更新发送状态为失败
			now := time.Now()
			errMsg := err.Error()
			s.emailRepo.UpdateEmailSendLogStatus(ctx, sendLog.ID, types.SendStatusFailed, &now, &errMsg)
			logger.Error("SendTimelockNotification SendEmail Error: ", err, "email", notification.Email)
		} else {
			// 更新发送状态为成功
			now := time.Now()
			s.emailRepo.UpdateEmailSendLogStatus(ctx, sendLog.ID, types.SendStatusSent, &now, nil)
			logger.Info("SendTimelockNotification SendEmail Success: ", "email", notification.Email, "event_type", eventType)
		}
	}

	logger.Info("SendTimelockNotification: ", "timelock_address", timelockAddress, "event_type", eventType, "email_count", len(notifications))
	return nil
}

// GetEmailSendLogs 获取邮件发送记录
func (s *service) GetEmailSendLogs(ctx context.Context, walletAddress string, page, pageSize int) ([]types.EmailSendLogResponse, int, error) {
	// 参数验证
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 获取数据
	logs, total, err := s.emailRepo.GetEmailSendLogsByWallet(ctx, walletAddress, page, pageSize)
	if err != nil {
		logger.Error("GetEmailSendLogs Error: ", err, "wallet_address", walletAddress)
		return nil, 0, fmt.Errorf("failed to get email send logs: %w", err)
	}

	// 转换为响应格式
	items := make([]types.EmailSendLogResponse, len(logs))
	for i, log := range logs {
		items[i] = types.EmailSendLogResponse{
			ID:              log.ID,
			Email:           log.Email,
			TimelockAddress: log.TimelockAddress,
			TransactionHash: log.TransactionHash,
			EventType:       log.EventType,
			Subject:         log.Subject,
			IsEmergency:     log.IsEmergency,
			IsReplied:       log.IsReplied,
			RepliedAt:       log.RepliedAt,
			SendStatus:      log.SendStatus,
			SentAt:          log.SentAt,
			CreatedAt:       log.CreatedAt,
		}
	}

	logger.Info("GetEmailSendLogs: ", "wallet_address", walletAddress, "total", total, "page", page)
	return items, total, nil
}

// ReplyEmergencyEmail 回复应急邮件
func (s *service) ReplyEmergencyEmail(ctx context.Context, token string) (*types.EmergencyReplyResponse, error) {
	// 处理应急邮件回复
	log, err := s.emailRepo.ReplyEmergencyEmail(ctx, token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEmergencyReplyNotFound
		}
		logger.Error("ReplyEmergencyEmail Error: ", err, "token", token)
		return nil, fmt.Errorf("failed to reply emergency email: %w", err)
	}

	logger.Info("ReplyEmergencyEmail: ", "id", log.ID, "email", log.Email, "replied_at", log.RepliedAt)
	return &types.EmergencyReplyResponse{
		Success:   true,
		Message:   "Emergency notification acknowledged successfully",
		RepliedAt: *log.RepliedAt,
	}, nil
}

// GetVerifiedEmailsByWallet 获取钱包地址下的已验证邮箱（用于timelock合约选择）
func (s *service) GetVerifiedEmailsByWallet(ctx context.Context, walletAddress string) ([]types.EmailNotificationResponse, error) {
	// 获取该钱包地址下的所有已验证邮箱
	notifications, _, err := s.emailRepo.GetEmailNotificationsByWallet(ctx, walletAddress, 1, 1000) // 获取所有
	if err != nil {
		logger.Error("GetVerifiedEmailsByWallet Error: ", err, "wallet_address", walletAddress)
		return nil, fmt.Errorf("failed to get verified emails: %w", err)
	}

	// 过滤已验证的邮箱
	var verifiedEmails []types.EmailNotificationResponse
	for _, notification := range notifications {
		if notification.IsVerified && notification.IsActive {
			var timelockContracts []string
			json.Unmarshal([]byte(notification.TimelockContracts), &timelockContracts)

			verifiedEmails = append(verifiedEmails, types.EmailNotificationResponse{
				ID:                notification.ID,
				Email:             notification.Email,
				EmailRemark:       notification.EmailRemark,
				TimelockContracts: timelockContracts,
				IsVerified:        notification.IsVerified,
				IsActive:          notification.IsActive,
				CreatedAt:         notification.CreatedAt,
				UpdatedAt:         notification.UpdatedAt,
			})
		}
	}

	logger.Info("GetVerifiedEmailsByWallet: ", "wallet_address", walletAddress, "count", len(verifiedEmails))
	return verifiedEmails, nil
}

// UpdateTimelockContracts 更新邮箱监听的timelock合约（用于timelock合约管理）
func (s *service) UpdateTimelockContracts(ctx context.Context, walletAddress string, email string, timelockContracts []string) error {
	// 获取邮件配置
	notification, err := s.emailRepo.GetEmailNotificationByWalletAndEmail(ctx, walletAddress, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEmailNotFound
		}
		logger.Error("UpdateTimelockContracts Get Error: ", err, "wallet_address", walletAddress, "email", email)
		return fmt.Errorf("failed to get email notification: %w", err)
	}

	// 检查是否已验证
	if !notification.IsVerified {
		logger.Error("UpdateTimelockContracts Error: ", ErrEmailNotVerified, "wallet_address", walletAddress, "email", email)
		return ErrEmailNotVerified
	}

	// 转换为JSON
	timelockContractsJSON, err := json.Marshal(timelockContracts)
	if err != nil {
		logger.Error("UpdateTimelockContracts JSON Marshal Error: ", err, "wallet_address", walletAddress, "email", email)
		return fmt.Errorf("failed to marshal timelock contracts: %w", err)
	}

	// 更新timelock合约列表
	notification.TimelockContracts = string(timelockContractsJSON)

	if err := s.emailRepo.UpdateEmailNotification(ctx, notification); err != nil {
		logger.Error("UpdateTimelockContracts Update Error: ", err, "wallet_address", walletAddress, "email", email)
		return fmt.Errorf("failed to update timelock contracts: %w", err)
	}

	logger.Info("UpdateTimelockContracts: ", "wallet_address", walletAddress, "email", email, "contracts_count", len(timelockContracts))
	return nil
}

// 辅助方法

// generateEmailSubject 生成邮件主题
func (s *service) generateEmailSubject(eventType string) string {
	switch eventType {
	case types.EventTypeProposalCreated:
		return "TimeLocker Alert - New Proposal Created"
	case types.EventTypeProposalCanceled:
		return "TimeLocker Alert - Proposal Canceled"
	case types.EventTypeReadyToExecute:
		return "TimeLocker Alert - Ready to Execute"
	case types.EventTypeExecuted:
		return "TimeLocker Alert - Transaction Executed"
	case types.EventTypeExpired:
		return "TimeLocker Alert - Transaction Expired"
	default:
		return "TimeLocker Notification"
	}
}

// generateEmailContent 生成邮件内容摘要
func (s *service) generateEmailContent(eventType string, timelockAddress string, transactionHash *string) string {
	content := fmt.Sprintf("Event: %s\nTimelock: %s", eventType, timelockAddress)
	if transactionHash != nil {
		content += fmt.Sprintf("\nTransaction: %s", *transactionHash)
	}
	content += fmt.Sprintf("\nTime: %s", time.Now().Format("2006-01-02 15:04:05 UTC"))
	return content
}

// checkTimelockEmergencyMode 检查timelock合约是否启用应急模式
func (s *service) checkTimelockEmergencyMode(ctx context.Context, timelockAddress string) (bool, error) {
	// 查询该timelock合约的应急模式设置
	isEmergency, err := s.emailRepo.CheckTimelockEmergencyMode(ctx, timelockAddress)
	if err != nil {
		logger.Warn("checkTimelockEmergencyMode: failed to check timelock emergency mode, using global setting", "timelock_address", timelockAddress, "error", err)
		// 如果查询失败，使用全局配置作为后备
		return s.config.EnableEmergencyMode, nil
	}

	logger.Info("checkTimelockEmergencyMode", "timelock_address", timelockAddress, "emergency_mode", isEmergency)
	return isEmergency, nil
}
