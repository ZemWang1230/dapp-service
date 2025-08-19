package email

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
	"timelocker-backend/internal/config"
	chainRepo "timelocker-backend/internal/repository/chain"
	"timelocker-backend/internal/repository/email"
	"timelocker-backend/internal/types"
	emailPkg "timelocker-backend/pkg/email"
	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/utils"

	"gorm.io/gorm"
)

// EmailService 邮箱服务接口
type EmailService interface {
	// 邮箱管理
	AddUserEmail(ctx context.Context, userID int64, emailAddr string, remark *string) (*types.UserEmailResponse, error)
	GetUserEmails(ctx context.Context, userID int64, page, pageSize int) (*types.EmailListResponse, error)
	UpdateEmailRemark(ctx context.Context, userEmailID int64, userID int64, remark *string) error
	DeleteUserEmail(ctx context.Context, userEmailID int64, userID int64) error

	// 邮箱验证
	SendVerificationCode(ctx context.Context, userEmailID int64, userID int64) error
	VerifyEmail(ctx context.Context, userEmailID int64, userID int64, code string) error
	// 基于 email 发送验证码（创建/复用未验证记录，允许备注更新）
	SendVerificationCodeByEmail(ctx context.Context, userID int64, emailAddr string, remark *string) error
	// 基于 email 校验验证码
	VerifyEmailByEmail(ctx context.Context, userID int64, emailAddr string, code string) error

	// 通知发送
	SendFlowNotification(ctx context.Context, standard string, chainID int, contractAddress string, flowID string, statusFrom, statusTo string, txHash *string, initiatorAddress string) error

	// 工具方法
	CleanExpiredCodes(ctx context.Context) error
}

// emailService 邮箱服务实现
type emailService struct {
	repo      email.EmailRepository
	chainRepo chainRepo.Repository
	config    *config.Config
	sender    *emailPkg.SMTPSender
}

// NewEmailService 创建邮箱服务实例
func NewEmailService(repo email.EmailRepository, chainRepo chainRepo.Repository, cfg *config.Config) EmailService {
	return &emailService{
		repo:      repo,
		chainRepo: chainRepo,
		config:    cfg,
		sender:    emailPkg.NewSMTPSender(&cfg.Email),
	}
}

// ===== 邮箱管理方法 =====
// AddUserEmail 添加用户邮箱
func (s *emailService) AddUserEmail(ctx context.Context, userID int64, emailAddr string, remark *string) (*types.UserEmailResponse, error) {
	// 获取或创建邮箱记录
	emailRecord, err := s.repo.GetOrCreateEmail(ctx, emailAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create email: %w", err)
	}

	// 检查用户是否已添加此邮箱
	exists, err := s.repo.CheckUserEmailExists(ctx, userID, emailRecord.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check user email exists: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("email already added by user")
	}

	// 添加用户邮箱关系
	userEmail, err := s.repo.AddUserEmail(ctx, userID, emailRecord.ID, remark)
	if err != nil {
		return nil, fmt.Errorf("failed to add user email: %w", err)
	}

	return &types.UserEmailResponse{
		ID:             userEmail.ID,
		Email:          emailAddr,
		Remark:         remark,
		IsVerified:     false,
		LastVerifiedAt: nil,
		CreatedAt:      userEmail.CreatedAt,
	}, nil
}

// GetUserEmails 获取用户邮箱
func (s *emailService) GetUserEmails(ctx context.Context, userID int64, page, pageSize int) (*types.EmailListResponse, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	userEmails, total, err := s.repo.GetUserEmails(ctx, userID, offset, pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get user emails: %w", err)
	}

	// 转换为响应格式
	emails := make([]types.UserEmailResponse, len(userEmails))
	for i, ue := range userEmails {
		emails[i] = types.UserEmailResponse{
			ID:             ue.ID,
			Email:          ue.Email.Email,
			Remark:         ue.Remark,
			IsVerified:     ue.IsVerified,
			LastVerifiedAt: ue.LastVerifiedAt,
			CreatedAt:      ue.CreatedAt,
		}
	}

	return &types.EmailListResponse{
		Emails: emails,
		Total:  total,
	}, nil
}

// UpdateEmailRemark 更新用户邮箱备注
func (s *emailService) UpdateEmailRemark(ctx context.Context, userEmailID int64, userID int64, remark *string) error {
	if remark != nil {
		trimmed := strings.TrimSpace(*remark)
		if len(trimmed) > 200 {
			return fmt.Errorf("remark too long")
		}
		remark = &trimmed
	}
	err := s.repo.UpdateUserEmailRemark(ctx, userEmailID, userID, remark)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user email not found")
		}
		return fmt.Errorf("failed to update email remark: %w", err)
	}
	return nil
}

// DeleteUserEmail 删除用户邮箱
func (s *emailService) DeleteUserEmail(ctx context.Context, userEmailID int64, userID int64) error {
	err := s.repo.DeleteUserEmail(ctx, userEmailID, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user email not found")
		}
		return fmt.Errorf("failed to delete user email: %w", err)
	}
	return nil
}

// ===== 邮箱验证方法 =====
// SendVerificationCode 发送验证码
func (s *emailService) SendVerificationCode(ctx context.Context, userEmailID int64, userID int64) error {
	// 获取用户邮箱信息
	userEmail, err := s.repo.GetUserEmailByID(ctx, userEmailID, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user email not found")
		}
		return fmt.Errorf("failed to get user email: %w", err)
	}

	// 检查是否已验证
	if userEmail.IsVerified {
		return fmt.Errorf("email already verified")
	}

	// 检查最近是否发送过验证码（防止频繁发送）
	latestCode, err := s.repo.GetLatestVerificationCode(ctx, userEmailID)
	if err == nil {
		// 检查是否在1分钟内发送过
		if time.Since(latestCode.SentAt) < time.Minute {
			return fmt.Errorf("verification code sent recently, please wait")
		}
	}

	// 生成6位数字验证码
	code, err := s.generateVerificationCode()
	if err != nil {
		return fmt.Errorf("failed to generate verification code: %w", err)
	}

	// 设置过期时间
	expiresAt := time.Now().Add(s.config.Email.VerificationCodeExpiry)

	// 保存验证码
	if err := s.repo.CreateVerificationCode(ctx, userEmailID, code, expiresAt); err != nil {
		return fmt.Errorf("failed to save verification code: %w", err)
	}

	// 发送邮件
	if err := s.sendVerificationEmail(userEmail.Email.Email, code); err != nil {
		logger.Error("Failed to send verification email", err, "email", userEmail.Email.Email)
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	logger.Info("Verification code sent", "email", userEmail.Email.Email, "userEmailID", userEmailID)
	return nil
}

// SendVerificationCodeByEmail 基于 email 发送验证码
func (s *emailService) SendVerificationCodeByEmail(ctx context.Context, userID int64, emailAddr string, remark *string) error {
	// 标准化
	emailAddr = strings.ToLower(strings.TrimSpace(emailAddr))
	if !utils.IsValidEmail(emailAddr) {
		return fmt.Errorf("invalid email format")
	}
	if remark != nil {
		trimmed := strings.TrimSpace(*remark)
		if len(trimmed) > 200 {
			return fmt.Errorf("remark too long")
		}
		remark = &trimmed
	}
	// 获取或创建邮箱记录
	emailRecord, err := s.repo.GetOrCreateEmail(ctx, emailAddr)
	if err != nil {
		return fmt.Errorf("failed to get or create email: %w", err)
	}
	// 获取或创建 user_email 未验证记录
	userEmail, err := s.repo.GetUserEmailByUserAndEmailID(ctx, userID, emailRecord.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			if userEmail, err = s.repo.AddUserEmail(ctx, userID, emailRecord.ID, remark); err != nil {
				return fmt.Errorf("failed to add user email: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get user email: %w", err)
		}
	} else {
		if userEmail.IsVerified {
			return fmt.Errorf("email already added by user")
		}
		if err := s.repo.UpdateUserEmailRemark(ctx, userEmail.ID, userID, remark); err != nil {
			return fmt.Errorf("failed to update remark: %w", err)
		}
	}
	return s.SendVerificationCode(ctx, userEmail.ID, userID)
}

// VerifyEmailByEmail 基于 email 校验验证码
func (s *emailService) VerifyEmailByEmail(ctx context.Context, userID int64, emailAddr string, code string) error {
	// 标准化
	emailAddr = strings.ToLower(strings.TrimSpace(emailAddr))
	code = strings.TrimSpace(code)
	if !utils.IsValidEmail(emailAddr) {
		return fmt.Errorf("invalid email format")
	}
	if !utils.IsValidVerificationCode(code) {
		return fmt.Errorf("invalid or expired verification code")
	}
	// 获取邮箱
	emailRecord, err := s.repo.GetEmailByAddress(ctx, emailAddr)
	if err != nil {
		return fmt.Errorf("failed to get email: %w", err)
	}
	// 获取 user_email
	userEmail, err := s.repo.GetUserEmailByUserAndEmailID(ctx, userID, emailRecord.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user email not found")
		}
		return fmt.Errorf("failed to get user email: %w", err)
	}
	return s.VerifyEmail(ctx, userEmail.ID, userID, code)
}

// VerifyEmail 验证邮箱
func (s *emailService) VerifyEmail(ctx context.Context, userEmailID int64, userID int64, code string) error {
	// 验证码验证
	if err := s.repo.VerifyCode(ctx, userEmailID, code); err != nil {
		return fmt.Errorf("failed to verify code: %w", err)
	}

	// 标记邮箱为已验证
	if err := s.repo.VerifyUserEmail(ctx, userEmailID, userID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user email not found")
		}
		return fmt.Errorf("failed to verify user email: %w", err)
	}

	logger.Info("Email verified successfully", "userEmailID", userEmailID, "userID", userID)
	return nil
}

// ===== 通知发送方法 =====
// SendFlowNotification 发送流程通知
func (s *emailService) SendFlowNotification(ctx context.Context, standard string, chainID int, contractAddress string, flowID string, statusFrom, statusTo string, txHash *string, initiatorAddress string) error {
	// 获取与合约相关用户的已验证邮箱列表
	emailIDs, err := s.repo.GetContractRelatedVerifiedEmailIDs(ctx, standard, chainID, contractAddress)
	if err != nil {
		logger.Error("Failed to get related verified emails", err,
			"standard", standard, "chainID", chainID, "contract", contractAddress,
			"statusTo", statusTo, "initiator", initiatorAddress)
		return fmt.Errorf("failed to get related verified emails: %w", err)
	}

	if len(emailIDs) == 0 {
		logger.Debug("No related verified emails found for notification",
			"standard", standard, "chainID", chainID, "contract", contractAddress,
			"statusTo", statusTo, "initiator", initiatorAddress)
		return nil
	}

	logger.Info("Found related verified emails for notification",
		"count", len(emailIDs), "standard", standard, "chainID", chainID,
		"contract", contractAddress, "statusTo", statusTo, "initiator", initiatorAddress)

	// 对每个邮箱发送通知
	for _, emailID := range emailIDs {
		// 检查是否已发送过此通知
		exists, err := s.repo.CheckSendLogExists(ctx, emailID, flowID, statusTo)
		if err != nil {
			logger.Error("Failed to check send log", err, "emailID", emailID, "flowID", flowID)
			continue
		}
		if exists {
			logger.Info("Notification already sent", "emailID", emailID, "flowID", flowID, "status", statusTo)
			continue
		}

		// 发送通知邮件
		if err := s.sendFlowNotificationEmail(ctx, emailID, standard, chainID, contractAddress, flowID, statusFrom, statusTo, txHash); err != nil {
			logger.Error("Failed to send notification email", err, "emailID", emailID, "flowID", flowID)

			// 记录发送失败日志
			sendLog := &types.EmailSendLog{
				EmailID:          emailID,
				FlowID:           flowID,
				TimelockStandard: standard,
				ChainID:          chainID,
				ContractAddress:  contractAddress,
				StatusFrom:       &statusFrom,
				StatusTo:         statusTo,
				TxHash:           txHash,
				SendStatus:       "failed",
				ErrorMessage:     func() *string { s := err.Error(); return &s }(),
				RetryCount:       0,
			}
			s.repo.CreateSendLog(ctx, sendLog)
			continue
		}

		// 记录发送成功日志
		sendLog := &types.EmailSendLog{
			EmailID:          emailID,
			FlowID:           flowID,
			TimelockStandard: standard,
			ChainID:          chainID,
			ContractAddress:  contractAddress,
			StatusFrom:       &statusFrom,
			StatusTo:         statusTo,
			TxHash:           txHash,
			SendStatus:       "success",
			RetryCount:       0,
		}
		if err := s.repo.CreateSendLog(ctx, sendLog); err != nil {
			logger.Error("Failed to create send log", err, "emailID", emailID, "flowID", flowID)
		}

		logger.Info("Flow notification sent", "emailID", emailID, "flowID", flowID, "status", statusTo)
	}

	return nil
}

// ===== 工具方法 =====
// CleanExpiredCodes 清理过期验证码
func (s *emailService) CleanExpiredCodes(ctx context.Context) error {
	return s.repo.CleanExpiredCodes(ctx)
}

// ===== 私有辅助方法 =====

// generateVerificationCode 生成6位数字验证码
func (s *emailService) generateVerificationCode() (string, error) {
	code := ""
	for i := 0; i < 6; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code += num.String()
	}
	return code, nil
}

// sendVerificationEmail 发送验证码邮件
func (s *emailService) sendVerificationEmail(toEmail, code string) error {
	subject := "TimeLocker - Verify Your Email Address"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>TimeLocker Email Verification</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background: linear-gradient(135deg, #0f0f23 0%%, #1a1a2e 100%%); color: #ffffff; min-height: 100vh;">
	<div style="max-width: 800px; margin: 0 auto; padding: 40px 20px;">
		<!-- Header -->
		<div style="text-align: center; margin-bottom: 60px;">
			<div style="background: linear-gradient(135deg, #6366f1 0%%, #8b5cf6 50%%, #ec4899 100%%); -webkit-background-clip: text; background-clip: text; -webkit-text-fill-color: transparent; font-size: 48px; font-weight: 800; margin-bottom: 16px; letter-spacing: -1px;">
				TimeLocker
			</div>
			<div style="height: 2px; width: 60px; background: linear-gradient(90deg, #6366f1, #8b5cf6, #ec4899); margin: 0 auto; border-radius: 1px;"></div>
		</div>

		<!-- Main Card -->
		<div style="background: rgba(255, 255, 255, 0.05); border-radius: 24px; padding: 60px; border: 1px solid rgba(255, 255, 255, 0.1); backdrop-filter: blur(20px);">
			<div style="text-align: center; margin-bottom: 50px;">
				<h1 style="color: #ffffff; font-size: 36px; font-weight: 700; margin: 0 0 16px 0; letter-spacing: -0.5px;">
					Verify Your Email
				</h1>
				<p style="color: #cbd5e1; font-size: 18px; line-height: 1.6; margin: 0; font-weight: 400;">
					Welcome to TimeLocker! Please use the verification code below to complete your email verification.
				</p>
			</div>

			<!-- Verification Code -->
			<div style="background: linear-gradient(135deg, #6366f1 0%%, #8b5cf6 50%%, #ec4899 100%%); border-radius: 20px; padding: 4px; margin: 50px 0;">
				<div style="background: rgba(15, 15, 35, 0.95); border-radius: 16px; padding: 50px; text-align: center;">
					<p style="color: #94a3b8; font-size: 16px; font-weight: 600; margin: 0 0 30px 0; text-transform: uppercase; letter-spacing: 2px;">
						Verification Code
					</p>
					<div style="color: #ffffff; font-size: 56px; font-weight: 800; font-family: 'Courier New', monospace; letter-spacing: 12px; margin: 0;">
						%s
					</div>
				</div>
			</div>

			<!-- Expiry Notice -->
			<div style="background: rgba(251, 146, 60, 0.1); border: 1px solid rgba(251, 146, 60, 0.2); border-radius: 16px; padding: 30px; margin: 50px 0; text-align: center;">
				<p style="color: #fed7aa; font-size: 16px; margin: 0; font-weight: 600;">
					This code will expire in <span style="color: #fb923c; font-weight: 700;">%s</span>
				</p>
			</div>

			<!-- Security Notice -->
			<div style="text-align: center; padding: 30px; background: rgba(71, 85, 105, 0.1); border-radius: 16px; border-left: 4px solid #6366f1;">
				<p style="color: #94a3b8; font-size: 14px; line-height: 1.6; margin: 0; font-weight: 400;">
					If you didn't request this verification, you can safely ignore this email. Your account remains secure.
				</p>
			</div>
		</div>

		<!-- Footer -->
		<div style="text-align: center; margin-top: 60px; padding-top: 40px; border-top: 1px solid rgba(255, 255, 255, 0.1);">
			<div style="color: #6366f1; font-size: 24px; font-weight: 700; margin-bottom: 12px;">TimeLocker</div>
			<p style="color: #64748b; font-size: 14px; margin: 0 0 8px 0; font-weight: 500;">
				This is an automated message from TimeLocker
			</p>
			<p style="color: #475569; font-size: 12px; margin: 0; font-weight: 400;">
				© 2025 TimeLocker Labs. All rights reserved.
			</p>
		</div>
	</div>
</body>
</html>
	`, code, s.config.Email.VerificationCodeExpiry.String())

	return s.sender.SendHTMLEmail(toEmail, subject, body)
}

// sendFlowNotificationEmail 发送流程通知邮件
func (s *emailService) sendFlowNotificationEmail(ctx context.Context, emailID int64, standard string, chainID int, contractAddress string, flowID string, statusFrom, statusTo string, txHash *string) error {
	// 获取邮箱地址
	emailRecord, err := s.getEmailByID(ctx, emailID)
	if err != nil {
		return fmt.Errorf("failed to get email: %w", err)
	}

	// 获取链信息
	chainInfo, err := s.chainRepo.GetChainByChainID(ctx, int64(chainID))
	if err != nil {
		logger.Error("Failed to get chain info", err, "chainID", chainID)
		return fmt.Errorf("failed to get chain info: %w", err)
	}

	// 解析区块浏览器URLs
	var explorerURLs []string
	if err := json.Unmarshal([]byte(chainInfo.BlockExplorerUrls), &explorerURLs); err != nil {
		logger.Error("Failed to parse block explorer URLs", err, "chainID", chainID)
		explorerURLs = []string{}
	}

	// 构建交易链接
	var txLink string
	var txDisplay string
	if txHash != nil && len(explorerURLs) > 0 {
		txLink = fmt.Sprintf("%s/tx/%s", explorerURLs[0], *txHash)
		// 简化显示的交易哈希（前6位...后4位）
		if len(*txHash) > 10 {
			txDisplay = fmt.Sprintf("%s...%s", (*txHash)[:6], (*txHash)[len(*txHash)-4:])
		} else {
			txDisplay = *txHash
		}
	} else {
		txDisplay = "Pending"
		txLink = ""
	}

	// 根据状态获取颜色
	getStatusColor := func(status string) (bgColor, textColor string) {
		switch strings.ToLower(status) {
		case "waiting":
			return "rgba(251, 146, 60, 0.15)", "#fb923c"
		case "ready":
			return "rgba(34, 197, 94, 0.15)", "#22c55e"
		case "executed":
			return "rgba(99, 102, 241, 0.15)", "#6366f1"
		case "cancelled":
			return "rgba(239, 68, 68, 0.15)", "#ef4444"
		case "expired":
			return "rgba(107, 114, 128, 0.15)", "#6b7280"
		default:
			return "rgba(148, 163, 184, 0.15)", "#94a3b8"
		}
	}

	fromBg, fromText := getStatusColor(statusFrom)
	toBg, toText := getStatusColor(statusTo)

	// 格式化状态变更
	statusChangeHTML := fmt.Sprintf(`
<div style="background: linear-gradient(135deg, #6366f1 0%%, #8b5cf6 50%%, #ec4899 100%%); border-radius: 20px; padding: 4px; margin: 40px 0;">
	<div style="background: rgba(15, 15, 35, 0.95); border-radius: 16px; padding: 40px; text-align: center;">
		<p style="color: #94a3b8; font-size: 14px; font-weight: 600; margin: 0 0 20px 0; text-transform: uppercase; letter-spacing: 2px;">Status Update</p>
		<div style="display: flex; align-items: center; justify-content: center; margin: 0; max-width: 400px; margin-left: auto; margin-right: auto;">
			<div style="background: %s; color: %s; padding: 16px 24px; border-radius: 12px; font-size: 16px; font-weight: 700; text-transform: uppercase; letter-spacing: 1px; min-width: 120px; text-align: center; box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);">%s</div>
			<div style="padding:0 20px; display:flex; align-items:center; justify-content:center; color:#ffffff; font-size:24px; font-weight:800; transform:translateY(6px);">→</div>
			<div style="background: %s; color: %s; padding: 16px 24px; border-radius: 12px; font-size: 16px; font-weight: 700; text-transform: uppercase; letter-spacing: 1px; min-width: 120px; text-align: center; box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);">%s</div>
		</div>
	</div>
</div>
`, fromBg, fromText, strings.Title(statusFrom), toBg, toText, strings.Title(statusTo))

	subject := fmt.Sprintf("TimeLocker Status Update: %s → %s", strings.Title(statusFrom), strings.Title(statusTo))

	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>TimeLocker Flow Notification</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background: linear-gradient(135deg, #0f0f23 0%%, #1a1a2e 100%%); color: #ffffff; min-height: 100vh;">
	<div style="max-width: 800px; margin: 0 auto; padding: 40px 20px;">
		<!-- Header -->
		<div style="text-align: center; margin-bottom: 60px;">
			<div style="background: linear-gradient(135deg, #6366f1 0%%, #8b5cf6 50%%, #ec4899 100%%); -webkit-background-clip: text; background-clip: text; -webkit-text-fill-color: transparent; font-size: 48px; font-weight: 800; margin-bottom: 16px; letter-spacing: -1px;">
				TimeLocker
			</div>
			<div style="height: 2px; width: 60px; background: linear-gradient(90deg, #6366f1, #8b5cf6, #ec4899); margin: 0 auto; border-radius: 1px;"></div>
		</div>

		<!-- Main Card -->
		<div style="background: rgba(255, 255, 255, 0.05); border-radius: 24px; padding: 60px; border: 1px solid rgba(255, 255, 255, 0.1); backdrop-filter: blur(20px);">
			<div style="text-align: center; margin-bottom: 50px;">
				<h1 style="color: #ffffff; font-size: 36px; font-weight: 700; margin: 0 0 16px 0; letter-spacing: -0.5px;">
					Flow Status Update
				</h1>
				<p style="color: #cbd5e1; font-size: 18px; line-height: 1.6; margin: 0; font-weight: 400;">
					Your subscribed timelock flow has a status update
				</p>
			</div>

			<!-- Status Change -->
			%s

			<!-- Details Section -->
			<div style="background: rgba(255, 255, 255, 0.08); border-radius: 20px; padding: 40px; margin: 50px 0;">
				<!-- Standard -->
				<div style="display: flex; align-items: center; padding: 20px 0; border-bottom: 1px solid rgba(255, 255, 255, 0.1);">
					<div style="color: #94a3b8; font-size: 18px; font-weight: 600;">Standard</div>
					<div style="margin-left: auto; background: linear-gradient(135deg, #6366f1, #8b5cf6); color: #ffffff; padding: 10px 20px; border-radius: 12px; font-weight: 700; font-size: 14px; text-transform: uppercase; letter-spacing: 1px;">%s</div>
				</div>
				<!-- Network -->
				<div style="display: flex; align-items: center; padding: 20px 0; border-bottom: 1px solid rgba(255, 255, 255, 0.1);">
					<div style="color: #94a3b8; font-size: 18px; font-weight: 600;">Network</div>
					<div style="margin-left: auto; color: #ffffff; font-weight: 700; font-size: 18px;">%s</div>
				</div>
				<!-- Contract -->
				<div style="display: flex; align-items: flex-start; padding: 20px 0; border-bottom: 1px solid rgba(255, 255, 255, 0.1);">
					<div style="color: #94a3b8; font-size: 18px; font-weight: 600; margin-top: 2px;">Contract</div>
					<div style="margin-left: auto; color: #e2e8f0; font-family: 'Courier New', monospace; font-size: 14px; font-weight: 600; text-align: right; word-break: break-all; overflow-wrap: break-word; line-height: 1.4; max-width: 500px;">%s</div>
				</div>
				<!-- Transaction -->
				<div style="display: flex; align-items: center; padding: 20px 0 0 0;">
					<div style="color: #94a3b8; font-size: 18px; font-weight: 600;">Transaction</div>
					<div style="margin-left: auto; text-align: right;">%s</div>
				</div>
			</div>

			<!-- Dashboard Button -->
			<div style="text-align: center; margin: 50px 0;">
				<a href="%s" style="display: inline-block; background: linear-gradient(135deg, #6366f1, #8b5cf6); color: #ffffff; padding: 18px 40px; text-decoration: none; border-radius: 12px; font-weight: 700; font-size: 18px; box-shadow: 0 10px 30px rgba(99, 102, 241, 0.3);">
					View Dashboard
				</a>
			</div>

			<!-- Notice -->
			<div style="text-align: center; padding: 30px; background: rgba(71, 85, 105, 0.1); border-radius: 16px; border-left: 4px solid #6366f1;">
				<p style="color: #94a3b8; font-size: 14px; line-height: 1.6; margin: 0; font-weight: 400;">
					You can manage your email subscriptions in the dashboard settings.
				</p>
			</div>
		</div>

		<!-- Footer -->
		<div style="text-align: center; margin-top: 60px; padding-top: 40px; border-top: 1px solid rgba(255, 255, 255, 0.1);">
			<div style="color: #6366f1; font-size: 24px; font-weight: 700; margin-bottom: 12px;">TimeLocker</div>
			<p style="color: #64748b; font-size: 14px; margin: 0 0 8px 0; font-weight: 500;">
				This is an automated notification from TimeLocker
			</p>
			<p style="color: #475569; font-size: 12px; margin: 0; font-weight: 400;">
				© 2025 TimeLocker Labs. All rights reserved.
			</p>
		</div>
	</div>
</body>
</html>
	`,
		statusChangeHTML,        // statusChangeHTML (1个参数)
		strings.Title(standard), // standard (1个参数)
		chainInfo.DisplayName,   // network (1个参数)
		contractAddress,         // contract (1个参数)
		func() string { // transaction (1个参数)
			if txLink != "" {
				return fmt.Sprintf(`<a href="%s" style="color: #6366f1; text-decoration: none; font-family: 'Courier New', monospace; font-size: 14px; font-weight: 600; background: rgba(99, 102, 241, 0.1); padding: 8px 12px; border-radius: 8px; border: 1px solid rgba(99, 102, 241, 0.3);" target="_blank">%s ↗</a>`, txLink, txDisplay)
			}
			return fmt.Sprintf(`<span style="color: #94a3b8; font-size: 14px; font-style: italic; font-weight: 500;">%s</span>`, txDisplay)
		}(),
		s.config.Email.EmailURL) // dashboard URL (1个参数)

	return s.sender.SendHTMLEmail(emailRecord.Email, subject, body)
}

// getEmailByID 根据ID获取邮箱记录
func (s *emailService) getEmailByID(ctx context.Context, emailID int64) (*types.Email, error) {
	return s.repo.GetEmailByID(ctx, emailID)
}
