package email

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/email"
	"timelocker-backend/internal/types"
	emailPkg "timelocker-backend/pkg/email"
	"timelocker-backend/pkg/logger"

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
	repo   email.EmailRepository
	config *config.Config
	sender *emailPkg.SMTPSender
}

// NewEmailService 创建邮箱服务实例
func NewEmailService(repo email.EmailRepository, cfg *config.Config) EmailService {
	return &emailService{
		repo:   repo,
		config: cfg,
		sender: emailPkg.NewSMTPSender(&cfg.Email),
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
	subject := "TimeLocker Email Verification"
	body := fmt.Sprintf(`
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h2 style="color: #2c3e50;">Email Verification</h2>
		<p>Hello,</p>
		<p>You have requested to verify your email address for TimeLocker. Please use the verification code below:</p>
		<div style="background: #f8f9fa; padding: 20px; margin: 20px 0; border-radius: 5px; text-align: center;">
			<h1 style="color: #007bff; font-size: 32px; margin: 0; letter-spacing: 5px;">%s</h1>
		</div>
		<p>This code will expire in %v.</p>
		<p>If you didn't request this verification, please ignore this email.</p>
		<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
		<p style="color: #666; font-size: 12px;">
			This is an automated message from TimeLocker. Please do not reply to this email.
		</p>
	</div>
</body>
</html>
	`, code, s.config.Email.VerificationCodeExpiry)

	return s.sender.SendHTMLEmail(toEmail, subject, body)
}

// sendFlowNotificationEmail 发送流程通知邮件
func (s *emailService) sendFlowNotificationEmail(ctx context.Context, emailID int64, standard string, chainID int, contractAddress string, flowID string, statusFrom, statusTo string, txHash *string) error {
	// 获取邮箱地址
	emailRecord, err := s.getEmailByID(ctx, emailID)
	if err != nil {
		return fmt.Errorf("failed to get email: %w", err)
	}

	subject := fmt.Sprintf("TimeLocker Flow Notification: %s → %s", strings.Title(statusFrom), strings.Title(statusTo))

	txHashDisplay := "N/A"
	if txHash != nil {
		txHashDisplay = *txHash
	}

	body := fmt.Sprintf(`
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h2 style="color: #2c3e50;">TimeLocker Flow Status Update</h2>
		<p>Hello,</p>
		<p>Your subscribed timelock flow has a status update:</p>
		
		<div style="background: #f8f9fa; padding: 20px; margin: 20px 0; border-radius: 5px;">
			<table style="width: 100%%; border-collapse: collapse;">
				<tr>
					<td style="padding: 8px 0; font-weight: bold;">Flow ID:</td>
					<td style="padding: 8px 0;">%s</td>
				</tr>
				<tr>
					<td style="padding: 8px 0; font-weight: bold;">Standard:</td>
					<td style="padding: 8px 0;">%s</td>
				</tr>
				<tr>
					<td style="padding: 8px 0; font-weight: bold;">Chain ID:</td>
					<td style="padding: 8px 0;">%d</td>
				</tr>
				<tr>
					<td style="padding: 8px 0; font-weight: bold;">Contract:</td>
					<td style="padding: 8px 0; font-family: monospace;">%s</td>
				</tr>
				<tr>
					<td style="padding: 8px 0; font-weight: bold;">Status Change:</td>
					<td style="padding: 8px 0;"><span style="color: #dc3545;">%s</span> → <span style="color: #28a745;">%s</span></td>
				</tr>
				<tr>
					<td style="padding: 8px 0; font-weight: bold;">Transaction:</td>
					<td style="padding: 8px 0; font-family: monospace;">%s</td>
				</tr>
			</table>
		</div>
		
		<p>You can view more details on the TimeLocker dashboard.</p>
		<p><a href="%s" style="background: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">View Dashboard</a></p>
		
		<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
		<p style="color: #666; font-size: 12px;">
			This is an automated notification from TimeLocker. You can manage your email subscriptions in the dashboard.
		</p>
	</div>
</body>
</html>
	`, flowID, strings.Title(standard), chainID, contractAddress, strings.Title(statusFrom), strings.Title(statusTo), txHashDisplay, s.config.Email.EmailURL)

	return s.sender.SendHTMLEmail(emailRecord.Email, subject, body)
}

// getEmailByID 根据ID获取邮箱记录
func (s *emailService) getEmailByID(ctx context.Context, emailID int64) (*types.Email, error) {
	return s.repo.GetEmailByID(ctx, emailID)
}
