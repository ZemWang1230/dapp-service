package email_notification

import (
	"context"
	"fmt"
	"time"

	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository 邮件通知仓库接口
type Repository interface {
	// 邮件通知配置相关
	CreateEmailNotification(ctx context.Context, notification *types.EmailNotification) error
	GetEmailNotificationByWalletAndEmail(ctx context.Context, walletAddress, email string) (*types.EmailNotification, error)
	GetEmailNotificationsByWallet(ctx context.Context, walletAddress string, page, pageSize int) ([]types.EmailNotification, int, error)
	GetEmailNotificationByID(ctx context.Context, id int64) (*types.EmailNotification, error)
	UpdateEmailNotification(ctx context.Context, notification *types.EmailNotification) error
	DeleteEmailNotification(ctx context.Context, id int64) error
	GetVerifiedEmailsByTimelockContract(ctx context.Context, timelockAddress string) ([]types.EmailNotification, error)

	// 验证码相关
	SetVerificationCode(ctx context.Context, walletAddress, email, code string, expiresAt time.Time) error
	VerifyEmailCode(ctx context.Context, walletAddress, email, code string) error
	ClearVerificationCode(ctx context.Context, walletAddress, email string) error

	// 邮件发送记录相关
	CreateEmailSendLog(ctx context.Context, log *types.EmailSendLog) error
	GetEmailSendLogsByTimelock(ctx context.Context, timelockAddress string, page, pageSize int) ([]types.EmailSendLog, int, error)
	GetEmailSendLogsByWallet(ctx context.Context, walletAddress string, page, pageSize int) ([]types.EmailSendLog, int, error)
	UpdateEmailSendLogStatus(ctx context.Context, id int64, status string, sentAt *time.Time, errorMsg *string) error
	GetPendingEmailSendLogs(ctx context.Context) ([]types.EmailSendLog, error)

	// 应急通知相关
	CreateEmergencyNotification(ctx context.Context, emergency *types.EmergencyNotification) error
	GetEmergencyNotification(ctx context.Context, timelockAddress, transactionHash string) (*types.EmergencyNotification, error)
	UpdateEmergencyNotificationReply(ctx context.Context, timelockAddress, transactionHash string) error
	GetPendingEmergencyNotifications(ctx context.Context) ([]types.EmergencyNotification, error)
	UpdateEmergencyNotificationNextSend(ctx context.Context, id int64, nextSendAt time.Time, sendCount int) error

	// 应急邮件回复相关
	ReplyEmergencyEmail(ctx context.Context, token string) (*types.EmailSendLog, error)
	GetEmailSendLogByToken(ctx context.Context, token string) (*types.EmailSendLog, error)

	// Timelock应急模式检查
	CheckTimelockEmergencyMode(ctx context.Context, timelockAddress string) (bool, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository 创建邮件通知仓库
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// CreateEmailNotification 创建邮件通知配置
func (r *repository) CreateEmailNotification(ctx context.Context, notification *types.EmailNotification) error {
	err := r.db.WithContext(ctx).Create(notification).Error
	if err != nil {
		logger.Error("CreateEmailNotification Error: ", err, "wallet_address", notification.WalletAddress, "email", notification.Email)
		return err
	}
	logger.Info("CreateEmailNotification: ", "id", notification.ID, "wallet_address", notification.WalletAddress, "email", notification.Email)
	return nil
}

// GetEmailNotificationByWalletAndEmail 根据钱包地址和邮箱获取配置
func (r *repository) GetEmailNotificationByWalletAndEmail(ctx context.Context, walletAddress, email string) (*types.EmailNotification, error) {
	var notification types.EmailNotification
	err := r.db.WithContext(ctx).
		Where("wallet_address = ? AND email = ?", walletAddress, email).
		First(&notification).Error

	if err != nil {
		logger.Error("GetEmailNotificationByWalletAndEmail Error: ", err, "wallet_address", walletAddress, "email", email)
		return nil, err
	}
	logger.Info("GetEmailNotificationByWalletAndEmail: ", "id", notification.ID, "wallet_address", notification.WalletAddress, "email", notification.Email)
	return &notification, nil
}

// GetEmailNotificationsByWallet 根据钱包地址获取邮件配置列表（分页）
func (r *repository) GetEmailNotificationsByWallet(ctx context.Context, walletAddress string, page, pageSize int) ([]types.EmailNotification, int, error) {
	var notifications []types.EmailNotification
	var total int64

	// 计算总数
	if err := r.db.WithContext(ctx).
		Model(&types.EmailNotification{}).
		Where("wallet_address = ?", walletAddress).
		Count(&total).Error; err != nil {
		logger.Error("GetEmailNotificationsByWallet Count Error: ", err, "wallet_address", walletAddress)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).
		Where("wallet_address = ?", walletAddress).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&notifications).Error

	if err != nil {
		logger.Error("GetEmailNotificationsByWallet Error: ", err, "wallet_address", walletAddress)
		return nil, 0, err
	}

	logger.Info("GetEmailNotificationsByWallet: ", "wallet_address", walletAddress, "total", total, "page", page)
	return notifications, int(total), nil
}

// GetEmailNotificationByID 根据ID获取邮件配置
func (r *repository) GetEmailNotificationByID(ctx context.Context, id int64) (*types.EmailNotification, error) {
	var notification types.EmailNotification
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&notification).Error

	if err != nil {
		logger.Error("GetEmailNotificationByID Error: ", err, "id", id)
		return nil, err
	}
	logger.Info("GetEmailNotificationByID: ", "id", notification.ID, "email", notification.Email)
	return &notification, nil
}

// UpdateEmailNotification 更新邮件通知配置
func (r *repository) UpdateEmailNotification(ctx context.Context, notification *types.EmailNotification) error {
	err := r.db.WithContext(ctx).
		Model(notification).
		Where("id = ?", notification.ID).
		Updates(notification).Error

	if err != nil {
		logger.Error("UpdateEmailNotification Error: ", err, "id", notification.ID)
		return err
	}
	logger.Info("UpdateEmailNotification: ", "id", notification.ID, "email", notification.Email)
	return nil
}

// DeleteEmailNotification 删除邮件通知配置
func (r *repository) DeleteEmailNotification(ctx context.Context, id int64) error {
	err := r.db.WithContext(ctx).
		Delete(&types.EmailNotification{}, id).Error

	if err != nil {
		logger.Error("DeleteEmailNotification Error: ", err, "id", id)
		return err
	}
	logger.Info("DeleteEmailNotification: ", "id", id)
	return nil
}

// GetVerifiedEmailsByTimelockContract 获取监听指定timelock合约的已验证邮箱
func (r *repository) GetVerifiedEmailsByTimelockContract(ctx context.Context, timelockAddress string) ([]types.EmailNotification, error) {
	var notifications []types.EmailNotification
	err := r.db.WithContext(ctx).
		Where("is_verified = ? AND is_active = ? AND timelock_contracts LIKE ?",
			true, true, fmt.Sprintf("%%%s%%", timelockAddress)).
		Find(&notifications).Error

	if err != nil {
		logger.Error("GetVerifiedEmailsByTimelockContract Error: ", err, "timelock_address", timelockAddress)
		return nil, err
	}
	logger.Info("GetVerifiedEmailsByTimelockContract: ", "timelock_address", timelockAddress, "count", len(notifications))
	return notifications, nil
}

// SetVerificationCode 设置验证码
func (r *repository) SetVerificationCode(ctx context.Context, walletAddress, email, code string, expiresAt time.Time) error {
	err := r.db.WithContext(ctx).
		Model(&types.EmailNotification{}).
		Where("wallet_address = ? AND email = ?", walletAddress, email).
		Updates(map[string]interface{}{
			"verification_code":       code,
			"verification_expires_at": expiresAt,
		}).Error

	if err != nil {
		logger.Error("SetVerificationCode Error: ", err, "wallet_address", walletAddress, "email", email)
		return err
	}
	logger.Info("SetVerificationCode: ", "wallet_address", walletAddress, "email", email, "expires_at", expiresAt)
	return nil
}

// VerifyEmailCode 验证邮箱验证码
func (r *repository) VerifyEmailCode(ctx context.Context, walletAddress, email, code string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&types.EmailNotification{}).
		Where("wallet_address = ? AND email = ? AND verification_code = ? AND verification_expires_at > ?",
			walletAddress, email, code, now).
		Updates(map[string]interface{}{
			"is_verified":             true,
			"verification_code":       nil,
			"verification_expires_at": nil,
		})

	if result.Error != nil {
		logger.Error("VerifyEmailCode Error: ", result.Error, "wallet_address", walletAddress, "email", email)
		return result.Error
	}

	if result.RowsAffected == 0 {
		logger.Error("VerifyEmailCode Error: ", fmt.Errorf("invalid or expired verification code"), "wallet_address", walletAddress, "email", email)
		return fmt.Errorf("invalid or expired verification code")
	}

	logger.Info("VerifyEmailCode: ", "wallet_address", walletAddress, "email", email)
	return nil
}

// ClearVerificationCode 清除验证码
func (r *repository) ClearVerificationCode(ctx context.Context, walletAddress, email string) error {
	err := r.db.WithContext(ctx).
		Model(&types.EmailNotification{}).
		Where("wallet_address = ? AND email = ?", walletAddress, email).
		Updates(map[string]interface{}{
			"verification_code":       nil,
			"verification_expires_at": nil,
		}).Error

	if err != nil {
		logger.Error("ClearVerificationCode Error: ", err, "wallet_address", walletAddress, "email", email)
		return err
	}
	logger.Info("ClearVerificationCode: ", "wallet_address", walletAddress, "email", email)
	return nil
}

// CreateEmailSendLog 创建邮件发送记录
func (r *repository) CreateEmailSendLog(ctx context.Context, log *types.EmailSendLog) error {
	err := r.db.WithContext(ctx).Create(log).Error
	if err != nil {
		logger.Error("CreateEmailSendLog Error: ", err, "email", log.Email, "timelock_address", log.TimelockAddress)
		return err
	}
	logger.Info("CreateEmailSendLog: ", "id", log.ID, "email", log.Email, "event_type", log.EventType)
	return nil
}

// GetEmailSendLogsByTimelock 根据timelock地址获取邮件发送记录（分页）
func (r *repository) GetEmailSendLogsByTimelock(ctx context.Context, timelockAddress string, page, pageSize int) ([]types.EmailSendLog, int, error) {
	var logs []types.EmailSendLog
	var total int64

	// 计算总数
	if err := r.db.WithContext(ctx).
		Model(&types.EmailSendLog{}).
		Where("timelock_address = ?", timelockAddress).
		Count(&total).Error; err != nil {
		logger.Error("GetEmailSendLogsByTimelock Count Error: ", err, "timelock_address", timelockAddress)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).
		Where("timelock_address = ?", timelockAddress).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&logs).Error

	if err != nil {
		logger.Error("GetEmailSendLogsByTimelock Error: ", err, "timelock_address", timelockAddress)
		return nil, 0, err
	}

	logger.Info("GetEmailSendLogsByTimelock: ", "timelock_address", timelockAddress, "total", total, "page", page)
	return logs, int(total), nil
}

// GetEmailSendLogsByWallet 根据钱包地址获取邮件发送记录（分页）
func (r *repository) GetEmailSendLogsByWallet(ctx context.Context, walletAddress string, page, pageSize int) ([]types.EmailSendLog, int, error) {
	var logs []types.EmailSendLog
	var total int64

	// 通过关联查询获取该钱包地址的邮件发送记录
	query := r.db.WithContext(ctx).
		Model(&types.EmailSendLog{}).
		Joins("JOIN email_notifications ON email_send_logs.email_notification_id = email_notifications.id").
		Where("email_notifications.wallet_address = ?", walletAddress)

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		logger.Error("GetEmailSendLogsByWallet Count Error: ", err, "wallet_address", walletAddress)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.
		Order("email_send_logs.created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&logs).Error

	if err != nil {
		logger.Error("GetEmailSendLogsByWallet Error: ", err, "wallet_address", walletAddress)
		return nil, 0, err
	}

	logger.Info("GetEmailSendLogsByWallet: ", "wallet_address", walletAddress, "total", total, "page", page)
	return logs, int(total), nil
}

// UpdateEmailSendLogStatus 更新邮件发送状态
func (r *repository) UpdateEmailSendLogStatus(ctx context.Context, id int64, status string, sentAt *time.Time, errorMsg *string) error {
	updates := map[string]interface{}{
		"send_status": status,
	}

	if sentAt != nil {
		updates["sent_at"] = sentAt
	}
	if errorMsg != nil {
		updates["error_message"] = errorMsg
	}

	err := r.db.WithContext(ctx).
		Model(&types.EmailSendLog{}).
		Where("id = ?", id).
		Updates(updates).Error

	if err != nil {
		logger.Error("UpdateEmailSendLogStatus Error: ", err, "id", id, "status", status)
		return err
	}
	logger.Info("UpdateEmailSendLogStatus: ", "id", id, "status", status)
	return nil
}

// GetPendingEmailSendLogs 获取待发送的邮件
func (r *repository) GetPendingEmailSendLogs(ctx context.Context) ([]types.EmailSendLog, error) {
	var logs []types.EmailSendLog
	err := r.db.WithContext(ctx).
		Where("send_status = ?", types.SendStatusPending).
		Order("created_at ASC").
		Find(&logs).Error

	if err != nil {
		logger.Error("GetPendingEmailSendLogs Error: ", err)
		return nil, err
	}
	logger.Info("GetPendingEmailSendLogs: ", "count", len(logs))
	return logs, nil
}

// CreateEmergencyNotification 创建应急通知记录
func (r *repository) CreateEmergencyNotification(ctx context.Context, emergency *types.EmergencyNotification) error {
	err := r.db.WithContext(ctx).Create(emergency).Error
	if err != nil {
		logger.Error("CreateEmergencyNotification Error: ", err, "timelock_address", emergency.TimelockAddress, "transaction_hash", emergency.TransactionHash)
		return err
	}
	logger.Info("CreateEmergencyNotification: ", "id", emergency.ID, "timelock_address", emergency.TimelockAddress)
	return nil
}

// GetEmergencyNotification 获取应急通知记录
func (r *repository) GetEmergencyNotification(ctx context.Context, timelockAddress, transactionHash string) (*types.EmergencyNotification, error) {
	var emergency types.EmergencyNotification
	err := r.db.WithContext(ctx).
		Where("timelock_address = ? AND transaction_hash = ?", timelockAddress, transactionHash).
		First(&emergency).Error

	if err != nil {
		logger.Error("GetEmergencyNotification Error: ", err, "timelock_address", timelockAddress, "transaction_hash", transactionHash)
		return nil, err
	}
	logger.Info("GetEmergencyNotification: ", "id", emergency.ID, "timelock_address", timelockAddress)
	return &emergency, nil
}

// UpdateEmergencyNotificationReply 更新应急通知回复状态
func (r *repository) UpdateEmergencyNotificationReply(ctx context.Context, timelockAddress, transactionHash string) error {
	err := r.db.WithContext(ctx).
		Model(&types.EmergencyNotification{}).
		Where("timelock_address = ? AND transaction_hash = ?", timelockAddress, transactionHash).
		Updates(map[string]interface{}{
			"replied_emails": gorm.Expr("replied_emails + 1"),
			"is_completed":   gorm.Expr("CASE WHEN replied_emails + 1 > 0 THEN true ELSE false END"),
		}).Error

	if err != nil {
		logger.Error("UpdateEmergencyNotificationReply Error: ", err, "timelock_address", timelockAddress, "transaction_hash", transactionHash)
		return err
	}
	logger.Info("UpdateEmergencyNotificationReply: ", "timelock_address", timelockAddress, "transaction_hash", transactionHash)
	return nil
}

// GetPendingEmergencyNotifications 获取待重发的应急通知
func (r *repository) GetPendingEmergencyNotifications(ctx context.Context) ([]types.EmergencyNotification, error) {
	var emergencies []types.EmergencyNotification
	now := time.Now()

	err := r.db.WithContext(ctx).
		Where("is_completed = ? AND next_send_at <= ?", false, now).
		Find(&emergencies).Error

	if err != nil {
		logger.Error("GetPendingEmergencyNotifications Error: ", err)
		return nil, err
	}
	logger.Info("GetPendingEmergencyNotifications: ", "count", len(emergencies))
	return emergencies, nil
}

// UpdateEmergencyNotificationNextSend 更新应急通知下次发送时间
func (r *repository) UpdateEmergencyNotificationNextSend(ctx context.Context, id int64, nextSendAt time.Time, sendCount int) error {
	err := r.db.WithContext(ctx).
		Model(&types.EmergencyNotification{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"next_send_at": nextSendAt,
			"send_count":   sendCount,
		}).Error

	if err != nil {
		logger.Error("UpdateEmergencyNotificationNextSend Error: ", err, "id", id)
		return err
	}
	logger.Info("UpdateEmergencyNotificationNextSend: ", "id", id, "next_send_at", nextSendAt, "send_count", sendCount)
	return nil
}

// ReplyEmergencyEmail 回复应急邮件
func (r *repository) ReplyEmergencyEmail(ctx context.Context, token string) (*types.EmailSendLog, error) {
	var log types.EmailSendLog
	now := time.Now()

	// 开启事务
	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 查找并更新邮件发送记录
	err := tx.Where("emergency_reply_token = ? AND is_emergency = ? AND is_replied = ?",
		token, true, false).
		First(&log).Error

	if err != nil {
		tx.Rollback()
		logger.Error("ReplyEmergencyEmail Find Error: ", err, "token", token)
		return nil, err
	}

	// 更新为已回复
	err = tx.Model(&log).Updates(map[string]interface{}{
		"is_replied": true,
		"replied_at": now,
	}).Error

	if err != nil {
		tx.Rollback()
		logger.Error("ReplyEmergencyEmail Update Error: ", err, "id", log.ID)
		return nil, err
	}

	// 更新应急通知状态
	if log.TransactionHash != nil {
		err = r.UpdateEmergencyNotificationReply(ctx, log.TimelockAddress, *log.TransactionHash)
		if err != nil {
			tx.Rollback()
			logger.Error("ReplyEmergencyEmail UpdateEmergency Error: ", err, "timelock_address", log.TimelockAddress)
			return nil, err
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logger.Error("ReplyEmergencyEmail Commit Error: ", err, "token", token)
		return nil, err
	}

	logger.Info("ReplyEmergencyEmail: ", "id", log.ID, "email", log.Email, "replied_at", now)
	return &log, nil
}

// GetEmailSendLogByToken 根据token获取邮件发送记录
func (r *repository) GetEmailSendLogByToken(ctx context.Context, token string) (*types.EmailSendLog, error) {
	var log types.EmailSendLog
	err := r.db.WithContext(ctx).
		Where("emergency_reply_token = ?", token).
		First(&log).Error

	if err != nil {
		logger.Error("GetEmailSendLogByToken Error: ", err, "token", token)
		return nil, err
	}
	logger.Info("GetEmailSendLogByToken: ", "id", log.ID, "email", log.Email)
	return &log, nil
}

// CheckTimelockEmergencyMode 检查timelock合约是否启用应急模式
func (r *repository) CheckTimelockEmergencyMode(ctx context.Context, timelockAddress string) (bool, error) {
	var emergencyMode bool

	// 查询Compound timelock合约的应急模式
	err := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Select("emergency_mode").
		Where("contract_address = ? AND status = 'active'", timelockAddress).
		First(&emergencyMode).Error

	if err == nil {
		logger.Info("CheckTimelockEmergencyMode: found compound timelock", "timelock_address", timelockAddress, "emergency_mode", emergencyMode)
		return emergencyMode, nil
	}

	// 如果不是Compound类型，检查OpenZeppelin timelock合约
	err = r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimeLock{}).
		Select("emergency_mode").
		Where("contract_address = ? AND status = 'active'", timelockAddress).
		First(&emergencyMode).Error

	if err == nil {
		logger.Info("CheckTimelockEmergencyMode: found openzeppelin timelock", "timelock_address", timelockAddress, "emergency_mode", emergencyMode)
		return emergencyMode, nil
	}

	// 如果都没找到，返回错误
	logger.Warn("CheckTimelockEmergencyMode: timelock contract not found", "timelock_address", timelockAddress)
	return false, fmt.Errorf("timelock contract not found: %s", timelockAddress)
}
