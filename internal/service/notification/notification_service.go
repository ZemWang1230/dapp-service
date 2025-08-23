package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"timelocker-backend/internal/config"
	chainRepo "timelocker-backend/internal/repository/chain"
	"timelocker-backend/internal/repository/notification"
	timelockRepo "timelocker-backend/internal/repository/timelock"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
	notificationPkg "timelocker-backend/pkg/notification"

	"gorm.io/gorm"
)

// NotificationService 通知服务接口
type NotificationService interface {
	// Telegram配置管理
	CreateTelegramConfig(ctx context.Context, userAddress string, req *types.CreateTelegramConfigRequest) (*types.TelegramConfig, error)
	GetTelegramConfigs(ctx context.Context, userAddress string) ([]*types.TelegramConfig, error)
	UpdateTelegramConfig(ctx context.Context, userAddress string, req *types.UpdateTelegramConfigRequest) error
	DeleteTelegramConfig(ctx context.Context, userAddress string, req *types.DeleteTelegramConfigRequest) error

	// Lark配置管理
	CreateLarkConfig(ctx context.Context, userAddress string, req *types.CreateLarkConfigRequest) (*types.LarkConfig, error)
	GetLarkConfigs(ctx context.Context, userAddress string) ([]*types.LarkConfig, error)
	UpdateLarkConfig(ctx context.Context, userAddress string, req *types.UpdateLarkConfigRequest) error
	DeleteLarkConfig(ctx context.Context, userAddress string, req *types.DeleteLarkConfigRequest) error

	// Feishu配置管理
	CreateFeishuConfig(ctx context.Context, userAddress string, req *types.CreateFeishuConfigRequest) (*types.FeishuConfig, error)
	GetFeishuConfigs(ctx context.Context, userAddress string) ([]*types.FeishuConfig, error)
	UpdateFeishuConfig(ctx context.Context, userAddress string, req *types.UpdateFeishuConfigRequest) error
	DeleteFeishuConfig(ctx context.Context, userAddress string, req *types.DeleteFeishuConfigRequest) error

	// 获取所有通知配置
	GetAllNotificationConfigs(ctx context.Context, userAddress string) (*types.NotificationConfigListResponse, error)

	// 通知发送
	SendFlowNotification(ctx context.Context, standard string, chainID int, contractAddress string, flowID string, statusFrom, statusTo string, txHash *string, initiatorAddress string) error
}

// notificationService 通知服务实现
type notificationService struct {
	repo           notification.NotificationRepository
	chainRepo      chainRepo.Repository
	timelockRepo   timelockRepo.Repository
	config         *config.Config
	telegramSender *notificationPkg.TelegramSender
	larkSender     *notificationPkg.LarkSender
	feishuSender   *notificationPkg.FeishuSender
}

// NewNotificationService 创建通知服务实例
func NewNotificationService(repo notification.NotificationRepository, chainRepo chainRepo.Repository, timelockRepo timelockRepo.Repository, config *config.Config) NotificationService {
	return &notificationService{
		repo:           repo,
		chainRepo:      chainRepo,
		timelockRepo:   timelockRepo,
		config:         config,
		telegramSender: notificationPkg.NewTelegramSender(),
		larkSender:     notificationPkg.NewLarkSender(),
		feishuSender:   notificationPkg.NewFeishuSender(),
	}
}

// ===== Telegram配置管理 =====
// CreateTelegramConfig 创建Telegram配置
func (s *notificationService) CreateTelegramConfig(ctx context.Context, userAddress string, req *types.CreateTelegramConfigRequest) (*types.TelegramConfig, error) {
	// 检查是否已存在同名配置
	existing, err := s.repo.GetTelegramConfigByUserAddressAndName(ctx, userAddress, req.Name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check existing telegram config: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("telegram config with name '%s' already exists", req.Name)
	}

	config := &types.TelegramConfig{
		UserAddress: userAddress,
		Name:        req.Name,
		BotToken:    req.BotToken,
		ChatID:      req.ChatID,
		IsActive:    true,
	}

	if err := s.repo.CreateTelegramConfig(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to create telegram config: %w", err)
	}

	return config, nil
}

// GetTelegramConfigs 获取Telegram配置
func (s *notificationService) GetTelegramConfigs(ctx context.Context, userAddress string) ([]*types.TelegramConfig, error) {
	return s.repo.GetTelegramConfigsByUserAddress(ctx, userAddress)
}

// UpdateTelegramConfig 更新Telegram配置
func (s *notificationService) UpdateTelegramConfig(ctx context.Context, userAddress string, req *types.UpdateTelegramConfigRequest) error {
	// 检查配置是否存在
	_, err := s.repo.GetTelegramConfigByUserAddressAndName(ctx, userAddress, *req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("telegram config not found")
		}
		return fmt.Errorf("failed to get telegram config: %w", err)
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if req.BotToken != nil {
		updates["bot_token"] = *req.BotToken
	}
	if req.ChatID != nil {
		updates["chat_id"] = *req.ChatID
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	return s.repo.UpdateTelegramConfig(ctx, userAddress, *req.Name, updates)
}

// DeleteTelegramConfig 删除Telegram配置
func (s *notificationService) DeleteTelegramConfig(ctx context.Context, userAddress string, req *types.DeleteTelegramConfigRequest) error {
	// 检查配置是否存在
	_, err := s.repo.GetTelegramConfigByUserAddressAndName(ctx, userAddress, req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("telegram config not found")
		}
		return fmt.Errorf("failed to get telegram config: %w", err)
	}

	return s.repo.DeleteTelegramConfig(ctx, userAddress, req.Name)
}

// ===== Lark配置管理 =====
// CreateLarkConfig 创建Lark配置
func (s *notificationService) CreateLarkConfig(ctx context.Context, userAddress string, req *types.CreateLarkConfigRequest) (*types.LarkConfig, error) {
	// 检查是否已存在同名配置
	existing, err := s.repo.GetLarkConfigByUserAddressAndName(ctx, userAddress, req.Name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check existing lark config: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("lark config with name '%s' already exists", req.Name)
	}

	config := &types.LarkConfig{
		UserAddress: userAddress,
		Name:        req.Name,
		WebhookURL:  req.WebhookURL,
		Secret:      req.Secret,
		IsActive:    true,
	}

	if err := s.repo.CreateLarkConfig(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to create lark config: %w", err)
	}

	return config, nil
}

// GetLarkConfigs 获取Lark配置
func (s *notificationService) GetLarkConfigs(ctx context.Context, userAddress string) ([]*types.LarkConfig, error) {
	return s.repo.GetLarkConfigsByUserAddress(ctx, userAddress)
}

// UpdateLarkConfig 更新Lark配置
func (s *notificationService) UpdateLarkConfig(ctx context.Context, userAddress string, req *types.UpdateLarkConfigRequest) error {
	// 检查配置是否存在
	_, err := s.repo.GetLarkConfigByUserAddressAndName(ctx, userAddress, *req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("lark config not found")
		}
		return fmt.Errorf("failed to get lark config: %w", err)
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if req.WebhookURL != nil {
		updates["webhook_url"] = *req.WebhookURL
	}
	if req.Secret != nil {
		updates["secret"] = *req.Secret
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	return s.repo.UpdateLarkConfig(ctx, userAddress, *req.Name, updates)
}

// DeleteLarkConfig 删除Lark配置
func (s *notificationService) DeleteLarkConfig(ctx context.Context, userAddress string, req *types.DeleteLarkConfigRequest) error {
	// 检查配置是否存在
	_, err := s.repo.GetLarkConfigByUserAddressAndName(ctx, userAddress, req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("lark config not found")
		}
		return fmt.Errorf("failed to get lark config: %w", err)
	}

	return s.repo.DeleteLarkConfig(ctx, userAddress, req.Name)
}

// ===== Feishu配置管理 =====
// CreateFeishuConfig 创建Feishu配置
func (s *notificationService) CreateFeishuConfig(ctx context.Context, userAddress string, req *types.CreateFeishuConfigRequest) (*types.FeishuConfig, error) {
	// 检查是否已存在同名配置
	existing, err := s.repo.GetFeishuConfigByUserAddressAndName(ctx, userAddress, req.Name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check existing feishu config: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("feishu config with name '%s' already exists", req.Name)
	}

	config := &types.FeishuConfig{
		UserAddress: userAddress,
		Name:        req.Name,
		WebhookURL:  req.WebhookURL,
		Secret:      req.Secret,
		IsActive:    true,
	}

	if err := s.repo.CreateFeishuConfig(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to create feishu config: %w", err)
	}

	return config, nil
}

// GetFeishuConfigs 获取Feishu配置
func (s *notificationService) GetFeishuConfigs(ctx context.Context, userAddress string) ([]*types.FeishuConfig, error) {
	return s.repo.GetFeishuConfigsByUserAddress(ctx, userAddress)
}

// UpdateFeishuConfig 更新Feishu配置
func (s *notificationService) UpdateFeishuConfig(ctx context.Context, userAddress string, req *types.UpdateFeishuConfigRequest) error {
	// 检查配置是否存在
	_, err := s.repo.GetFeishuConfigByUserAddressAndName(ctx, userAddress, *req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("feishu config not found")
		}
		return fmt.Errorf("failed to get feishu config: %w", err)
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if req.WebhookURL != nil {
		updates["webhook_url"] = *req.WebhookURL
	}
	if req.Secret != nil {
		updates["secret"] = *req.Secret
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	return s.repo.UpdateFeishuConfig(ctx, userAddress, *req.Name, updates)
}

// DeleteFeishuConfig 删除Feishu配置
func (s *notificationService) DeleteFeishuConfig(ctx context.Context, userAddress string, req *types.DeleteFeishuConfigRequest) error {
	// 检查配置是否存在
	_, err := s.repo.GetFeishuConfigByUserAddressAndName(ctx, userAddress, req.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("feishu config not found")
		}
		return fmt.Errorf("failed to get feishu config: %w", err)
	}

	return s.repo.DeleteFeishuConfig(ctx, userAddress, req.Name)
}

// ===== 获取所有通知配置 =====
// GetAllNotificationConfigs 获取所有通知配置
func (s *notificationService) GetAllNotificationConfigs(ctx context.Context, userAddress string) (*types.NotificationConfigListResponse, error) {
	response := &types.NotificationConfigListResponse{}

	// 获取Telegram配置
	telegramConfigs, err := s.repo.GetTelegramConfigsByUserAddress(ctx, userAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get telegram configs: %w", err)
	}
	response.TelegramConfigs = telegramConfigs

	// 获取Lark配置
	larkConfigs, err := s.repo.GetLarkConfigsByUserAddress(ctx, userAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get lark configs: %w", err)
	}
	response.LarkConfigs = larkConfigs

	// 获取Feishu配置
	feishuConfigs, err := s.repo.GetFeishuConfigsByUserAddress(ctx, userAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get feishu configs: %w", err)
	}
	response.FeishuConfigs = feishuConfigs

	return response, nil
}

// ===== 通知发送 =====
// SendFlowNotification 发送通知
func (s *notificationService) SendFlowNotification(ctx context.Context, standard string, chainID int, contractAddress string, flowID string, statusFrom, statusTo string, txHash *string, initiatorAddress string) error {
	// 获取与合约相关的所有用户地址
	userAddresses, err := s.repo.GetContractRelatedUserAddresses(ctx, standard, chainID, contractAddress)
	if err != nil {
		logger.Error("Failed to get contract related users", err, "standard", standard, "chainID", chainID, "contract", contractAddress)
		return nil // 不阻塞流程，只记录错误
	}

	if len(userAddresses) == 0 {
		logger.Debug("No related users found for notification", "standard", standard, "chainID", chainID, "contract", contractAddress)
		return nil
	}

	logger.Info("Found related users for notification", "count", len(userAddresses), "standard", standard, "chainID", chainID, "contract", contractAddress)

	// 生成通知消息
	message, err := s.generateNotificationMessage(ctx, standard, chainID, contractAddress, flowID, statusFrom, statusTo, txHash)
	if err != nil {
		logger.Error("Failed to generate notification message", err, "flowID", flowID)
		return nil // 不阻塞流程，只记录错误
	}

	// 对每个相关用户发送通知
	var totalSent int
	for _, userAddress := range userAddresses {
		// 获取用户的通知配置
		configs, err := s.repo.GetUserActiveNotificationConfigs(ctx, userAddress)
		if err != nil {
			logger.Error("Failed to get user notification configs", err, "userAddress", userAddress)
			continue // 继续处理下一个用户
		}

		// 检查是否有激活的配置
		totalConfigs := len(configs.TelegramConfigs) + len(configs.LarkConfigs) + len(configs.FeishuConfigs)
		if totalConfigs == 0 {
			logger.Debug("No active notification configs found", "userAddress", userAddress)
			continue
		}

		logger.Debug("Processing user notification configs", "userAddress", userAddress, "telegram", len(configs.TelegramConfigs), "lark", len(configs.LarkConfigs), "feishu", len(configs.FeishuConfigs))

		// 发送Telegram通知
		for _, config := range configs.TelegramConfigs {
			s.sendTelegramNotification(ctx, config, message, flowID, standard, chainID, contractAddress, statusFrom, statusTo, txHash)
			totalSent++
		}

		// 发送Lark通知
		for _, config := range configs.LarkConfigs {
			s.sendLarkNotification(ctx, config, message, flowID, standard, chainID, contractAddress, statusFrom, statusTo, txHash)
			totalSent++
		}

		// 发送Feishu通知
		for _, config := range configs.FeishuConfigs {
			s.sendFeishuNotification(ctx, config, message, flowID, standard, chainID, contractAddress, statusFrom, statusTo, txHash)
			totalSent++
		}
	}

	logger.Info("Notification sending completed", "totalUsers", len(userAddresses), "totalNotificationsSent", totalSent)
	return nil
}

// generateNotificationMessage 生成通知消息
func (s *notificationService) generateNotificationMessage(ctx context.Context, standard string, chainID int, contractAddress string, flowID string, statusFrom, statusTo string, txHash *string) (string, error) {
	// 获取链信息
	chain, err := s.chainRepo.GetChainByChainID(ctx, int64(chainID))
	if err != nil {
		return "", fmt.Errorf("failed to get chain info: %w", err)
	}

	// 获取状态表情符号
	getStatusEmoji := func(status string) string {
		switch strings.ToLower(status) {
		case "waiting":
			return "⏳"
		case "ready":
			return "✅"
		case "executed":
			return "🎯"
		case "cancelled":
			return "❌"
		case "expired":
			return "⏰"
		default:
			return "📋"
		}
	}

	// 构建简约消息
	message := fmt.Sprintf("━━━━━━━━━━━━━━━━\n")
	message += fmt.Sprintf("⚡ TimeLocker Notification\n")
	message += fmt.Sprintf("━━━━━━━━━━━━━━━━\n")
	message += fmt.Sprintf("[%s] %s    ➡️    [%s] %s\n", strings.ToUpper(statusFrom), getStatusEmoji(statusFrom), strings.ToUpper(statusTo), getStatusEmoji(statusTo))
	message += fmt.Sprintf("🔗 Chain    : %s\n", chain.DisplayName)
	message += fmt.Sprintf("📄 Contract : %s\n", contractAddress)
	message += fmt.Sprintf("⚙️ Standard : %s\n", strings.ToUpper(standard))

	// 添加交易链接
	if txHash != nil {
		if chain.BlockExplorerUrls != "" {
			var explorerUrls []string
			if err := json.Unmarshal([]byte(chain.BlockExplorerUrls), &explorerUrls); err == nil && len(explorerUrls) > 0 {
				message += fmt.Sprintf("🔍 Tx Hash  : %s", fmt.Sprintf("%s/tx/%s", explorerUrls[0], *txHash))
			}
		} else {
			message += fmt.Sprintf("🔍 Tx Hash  : %s", *txHash)
		}
	}

	logger.Info("Generated notification message", "flowID", flowID, "statusFrom", statusFrom, "statusTo", statusTo, "txHash", txHash)
	return message, nil
}

// sendTelegramNotification 发送Telegram通知
func (s *notificationService) sendTelegramNotification(ctx context.Context, config *types.TelegramConfig, message, flowID, standard string, chainID int, contractAddress, statusFrom, statusTo string, txHash *string) {
	// 检查是否已发送过此通知
	exists, err := s.repo.CheckNotificationLogExists(ctx, types.ChannelTelegram, config.UserAddress, config.ID, flowID, statusTo)
	if err != nil {
		logger.Error("Failed to check telegram notification log", err, "configID", config.ID, "flowID", flowID)
		return
	}
	if exists {
		logger.Info("Telegram notification already sent", "configID", config.ID, "flowID", flowID, "status", statusTo)
		return
	}

	// 发送消息
	err = s.telegramSender.SendMessage(config.BotToken, config.ChatID, message)
	sendStatus := "success"
	var errorMessage *string
	if err != nil {
		sendStatus = "failed"
		errMsg := err.Error()
		errorMessage = &errMsg
		logger.Error("Failed to send telegram notification", err, "configID", config.ID, "flowID", flowID)
	}

	// 记录发送日志
	log := &types.NotificationLog{
		UserAddress:      config.UserAddress,
		Channel:          types.ChannelTelegram,
		ConfigID:         config.ID,
		FlowID:           flowID,
		TimelockStandard: standard,
		ChainID:          chainID,
		ContractAddress:  contractAddress,
		StatusFrom:       statusFrom,
		StatusTo:         statusTo,
		TxHash: func() string {
			if txHash != nil {
				return *txHash
			}
			return ""
		}(),
		SendStatus: sendStatus,
		ErrorMessage: func() string {
			if errorMessage != nil {
				return *errorMessage
			}
			return ""
		}(),
		SentAt: time.Now(),
	}

	if err := s.repo.CreateNotificationLog(ctx, log); err != nil {
		logger.Error("Failed to create telegram notification log", err, "configID", config.ID, "flowID", flowID)
	}

	if sendStatus == "success" {
		logger.Info("Telegram notification sent", "configID", config.ID, "flowID", flowID, "status", statusTo)
	}
}

// sendLarkNotification 发送Lark通知
func (s *notificationService) sendLarkNotification(ctx context.Context, config *types.LarkConfig, message, flowID, standard string, chainID int, contractAddress, statusFrom, statusTo string, txHash *string) {
	// 检查是否已发送过此通知
	exists, err := s.repo.CheckNotificationLogExists(ctx, types.ChannelLark, config.UserAddress, config.ID, flowID, statusTo)
	if err != nil {
		logger.Error("Failed to check lark notification log", err, "configID", config.ID, "flowID", flowID)
		return
	}
	if exists {
		logger.Info("Lark notification already sent", "configID", config.ID, "flowID", flowID, "status", statusTo)
		return
	}

	// 发送消息
	err = s.larkSender.SendMessage(config.WebhookURL, config.Secret, message)
	sendStatus := "success"
	var errorMessage *string
	if err != nil {
		sendStatus = "failed"
		errMsg := err.Error()
		errorMessage = &errMsg
		logger.Error("Failed to send lark notification", err, "configID", config.ID, "flowID", flowID)
	}

	// 记录发送日志
	log := &types.NotificationLog{
		UserAddress:      config.UserAddress,
		Channel:          types.ChannelLark,
		ConfigID:         config.ID,
		FlowID:           flowID,
		TimelockStandard: standard,
		ChainID:          chainID,
		ContractAddress:  contractAddress,
		StatusFrom:       statusFrom,
		StatusTo:         statusTo,
		TxHash: func() string {
			if txHash != nil {
				return *txHash
			}
			return ""
		}(),
		SendStatus: sendStatus,
		ErrorMessage: func() string {
			if errorMessage != nil {
				return *errorMessage
			}
			return ""
		}(),
		SentAt: time.Now(),
	}

	if err := s.repo.CreateNotificationLog(ctx, log); err != nil {
		logger.Error("Failed to create lark notification log", err, "configID", config.ID, "flowID", flowID)
	}

	if sendStatus == "success" {
		logger.Info("Lark notification sent", "configID", config.ID, "flowID", flowID, "status", statusTo)
	}
}

// sendFeishuNotification 发送Feishu通知
func (s *notificationService) sendFeishuNotification(ctx context.Context, config *types.FeishuConfig, message, flowID, standard string, chainID int, contractAddress, statusFrom, statusTo string, txHash *string) {
	// 检查是否已发送过此通知
	exists, err := s.repo.CheckNotificationLogExists(ctx, types.ChannelFeishu, config.UserAddress, config.ID, flowID, statusTo)
	if err != nil {
		logger.Error("Failed to check feishu notification log", err, "configID", config.ID, "flowID", flowID)
		return
	}
	if exists {
		logger.Info("Feishu notification already sent", "configID", config.ID, "flowID", flowID, "status", statusTo)
		return
	}

	// 发送消息
	err = s.feishuSender.SendMessage(config.WebhookURL, config.Secret, message)
	sendStatus := "success"
	var errorMessage *string
	if err != nil {
		sendStatus = "failed"
		errMsg := err.Error()
		errorMessage = &errMsg
		logger.Error("Failed to send feishu notification", err, "configID", config.ID, "flowID", flowID)
	}

	// 记录发送日志
	log := &types.NotificationLog{
		UserAddress:      config.UserAddress,
		Channel:          types.ChannelFeishu,
		ConfigID:         config.ID,
		FlowID:           flowID,
		TimelockStandard: standard,
		ChainID:          chainID,
		ContractAddress:  contractAddress,
		StatusFrom:       statusFrom,
		StatusTo:         statusTo,
		TxHash: func() string {
			if txHash != nil {
				return *txHash
			}
			return ""
		}(),
		SendStatus: sendStatus,
		ErrorMessage: func() string {
			if errorMessage != nil {
				return *errorMessage
			}
			return ""
		}(),
		SentAt: time.Now(),
	}

	if err := s.repo.CreateNotificationLog(ctx, log); err != nil {
		logger.Error("Failed to create feishu notification log", err, "configID", config.ID, "flowID", flowID)
	}

	if sendStatus == "success" {
		logger.Info("Feishu notification sent", "configID", config.ID, "flowID", flowID, "status", statusTo)
	}
}
