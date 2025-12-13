package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"timelocker-backend/internal/config"
	chainRepo "timelocker-backend/internal/repository/chain"
	goldskyRepo "timelocker-backend/internal/repository/goldsky"
	"timelocker-backend/internal/repository/notification"
	timelockRepo "timelocker-backend/internal/repository/timelock"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
	notificationPkg "timelocker-backend/pkg/notification"
	"timelocker-backend/pkg/utils"

	"gorm.io/gorm"
)

// NotificationService 通知服务接口
type NotificationService interface {
	// 通用配置管理
	CreateNotificationConfig(ctx context.Context, userAddress string, req *types.CreateNotificationRequest) error
	UpdateNotificationConfig(ctx context.Context, userAddress string, req *types.UpdateNotificationRequest) error
	DeleteNotificationConfig(ctx context.Context, userAddress string, req *types.DeleteNotificationRequest) error

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
	flowRepo       goldskyRepo.FlowRepository
	config         *config.Config
	telegramSender *notificationPkg.TelegramSender
	larkSender     *notificationPkg.LarkSender
	feishuSender   *notificationPkg.FeishuSender
	discordSender  *notificationPkg.DiscordSender
	slackSender    *notificationPkg.SlackSender
}

// NewNotificationService 创建通知服务实例
func NewNotificationService(repo notification.NotificationRepository, chainRepo chainRepo.Repository, timelockRepo timelockRepo.Repository, flowRepo goldskyRepo.FlowRepository, config *config.Config) NotificationService {
	return &notificationService{
		repo:           repo,
		chainRepo:      chainRepo,
		timelockRepo:   timelockRepo,
		flowRepo:       flowRepo,
		config:         config,
		telegramSender: notificationPkg.NewTelegramSender(),
		larkSender:     notificationPkg.NewLarkSender(),
		feishuSender:   notificationPkg.NewFeishuSender(),
		discordSender:  notificationPkg.NewDiscordSender(),
		slackSender:    notificationPkg.NewSlackSender(),
	}
}

// ===== 通用配置管理 =====
// CreateNotificationConfig 创建通知配置
func (s *notificationService) CreateNotificationConfig(ctx context.Context, userAddress string, req *types.CreateNotificationRequest) error {
	switch strings.ToLower(req.Channel) {
	case "telegram":
		if req.BotToken == "" || req.ChatID == "" {
			return fmt.Errorf("bot_token and chat_id are required")
		}
		err := s.createTelegramConfig(ctx, userAddress, req.Name, req.BotToken, req.ChatID)
		if err != nil {
			return err
		}
		return nil

	case "lark":
		if req.WebhookURL == "" {
			return fmt.Errorf("webhook_url are required")
		}
		err := s.createLarkConfig(ctx, userAddress, req.Name, req.WebhookURL, req.Secret)
		if err != nil {
			return err
		}
		return nil

	case "feishu":
		if req.WebhookURL == "" {
			return fmt.Errorf("webhook_url are required")
		}
		err := s.createFeishuConfig(ctx, userAddress, req.Name, req.WebhookURL, req.Secret)
		if err != nil {
			return err
		}
		return nil
	case "discord":
		if req.WebhookURL == "" {
			return fmt.Errorf("webhook_url are required")
		}
		err := s.createDiscordConfig(ctx, userAddress, req.Name, req.WebhookURL)
		if err != nil {
			return err
		}
		return nil
	case "slack":
		if req.WebhookURL == "" {
			return fmt.Errorf("webhook_url are required")
		}
		err := s.createSlackConfig(ctx, userAddress, req.Name, req.WebhookURL)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("invalid channel: %s", req.Channel)
}

// UpdateNotificationConfig 更新通知配置
// 不需要更新的字段可以不填
func (s *notificationService) UpdateNotificationConfig(ctx context.Context, userAddress string, req *types.UpdateNotificationRequest) error {
	switch strings.ToLower(*req.Channel) {
	case "telegram":
		if req.BotToken == nil && req.ChatID == nil && req.IsActive == nil {
			return fmt.Errorf("at least one field must be provided")
		}
		return s.updateTelegramConfig(ctx, userAddress, req.Name, req.BotToken, req.ChatID, req.IsActive)
	case "lark":
		if req.WebhookURL == nil && req.Secret == nil && req.IsActive == nil {
			return fmt.Errorf("at least one field must be provided")
		}
		return s.updateLarkConfig(ctx, userAddress, req.Name, req.WebhookURL, req.Secret, req.IsActive)
	case "feishu":
		if req.WebhookURL == nil && req.Secret == nil && req.IsActive == nil {
			return fmt.Errorf("at least one field must be provided")
		}
		return s.updateFeishuConfig(ctx, userAddress, req.Name, req.WebhookURL, req.Secret, req.IsActive)
	case "discord":
		if req.WebhookURL == nil && req.IsActive == nil {
			return fmt.Errorf("at least one field must be provided")
		}
		return s.updateDiscordConfig(ctx, userAddress, req.Name, req.WebhookURL, req.IsActive)
	case "slack":
		if req.WebhookURL == nil && req.IsActive == nil {
			return fmt.Errorf("at least one field must be provided")
		}
		return s.updateSlackConfig(ctx, userAddress, req.Name, req.WebhookURL, req.IsActive)
	}
	return fmt.Errorf("invalid channel: %s", *req.Channel)
}

// DeleteNotificationConfig 删除通知配置
func (s *notificationService) DeleteNotificationConfig(ctx context.Context, userAddress string, req *types.DeleteNotificationRequest) error {
	switch strings.ToLower(req.Channel) {
	case "telegram":
		return s.deleteTelegramConfig(ctx, userAddress, req.Name)
	case "lark":
		return s.deleteLarkConfig(ctx, userAddress, req.Name)
	case "feishu":
		return s.deleteFeishuConfig(ctx, userAddress, req.Name)
	case "discord":
		return s.deleteDiscordConfig(ctx, userAddress, req.Name)
	case "slack":
		return s.deleteSlackConfig(ctx, userAddress, req.Name)
	}
	return fmt.Errorf("invalid channel: %s", req.Channel)
}

// ===== 创建配置 =====
// createTelegramConfig 创建Telegram配置
func (s *notificationService) createTelegramConfig(ctx context.Context, userAddress string, name string, botToken string, chatID string) error {
	// 检查是否已存在同名配置
	existing, err := s.repo.GetTelegramConfigByUserAddressAndName(ctx, userAddress, name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing telegram config: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("telegram config with name '%s' already exists", name)
	}

	config := &types.TelegramConfig{
		UserAddress: userAddress,
		Name:        name,
		BotToken:    botToken,
		ChatID:      chatID,
		IsActive:    true,
	}

	if err := s.repo.CreateTelegramConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to create telegram config: %w", err)
	}

	return nil
}

// createLarkConfig 创建Lark配置
func (s *notificationService) createLarkConfig(ctx context.Context, userAddress string, name string, webhookURL string, secret string) error {
	// 检查是否已存在同名配置
	existing, err := s.repo.GetLarkConfigByUserAddressAndName(ctx, userAddress, name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing lark config: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("lark config with name '%s' already exists", name)
	}

	config := &types.LarkConfig{
		UserAddress: userAddress,
		Name:        name,
		WebhookURL:  webhookURL,
		Secret:      secret,
		IsActive:    true,
	}

	if err := s.repo.CreateLarkConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to create lark config: %w", err)
	}

	return nil
}

// createFeishuConfig 创建Feishu配置
func (s *notificationService) createFeishuConfig(ctx context.Context, userAddress string, name string, webhookURL string, secret string) error {
	// 检查是否已存在同名配置
	existing, err := s.repo.GetFeishuConfigByUserAddressAndName(ctx, userAddress, name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing feishu config: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("feishu config with name '%s' already exists", name)
	}

	config := &types.FeishuConfig{
		UserAddress: userAddress,
		Name:        name,
		WebhookURL:  webhookURL,
		Secret:      secret,
		IsActive:    true,
	}

	if err := s.repo.CreateFeishuConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to create feishu config: %w", err)
	}

	return nil
}

// createDiscordConfig 创建Discord配置
func (s *notificationService) createDiscordConfig(ctx context.Context, userAddress string, name string, webhookURL string) error {
	// 检查是否已存在同名配置
	existing, err := s.repo.GetDiscordConfigByUserAddressAndName(ctx, userAddress, name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing discord config: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("discord config with name '%s' already exists", name)
	}

	config := &types.DiscordConfig{
		UserAddress: userAddress,
		Name:        name,
		WebhookURL:  webhookURL,
		IsActive:    true,
	}

	if err := s.repo.CreateDiscordConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to create discord config: %w", err)
	}

	return nil
}

// createSlackConfig 创建Slack配置
func (s *notificationService) createSlackConfig(ctx context.Context, userAddress string, name string, webhookURL string) error {
	// 检查是否已存在同名配置
	existing, err := s.repo.GetSlackConfigByUserAddressAndName(ctx, userAddress, name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing slack config: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("slack config with name '%s' already exists", name)
	}

	config := &types.SlackConfig{
		UserAddress: userAddress,
		Name:        name,
		WebhookURL:  webhookURL,
		IsActive:    true,
	}

	if err := s.repo.CreateSlackConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to create slack config: %w", err)
	}

	return nil
}

// ===== 更新配置 =====
// updateTelegramConfig 更新Telegram配置
func (s *notificationService) updateTelegramConfig(ctx context.Context, userAddress string, name *string, botToken *string, chatID *string, isActive *bool) error {
	// 检查配置是否存在
	_, err := s.repo.GetTelegramConfigByUserAddressAndName(ctx, userAddress, *name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("telegram config not found")
		}
		return fmt.Errorf("failed to get telegram config: %w", err)
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if botToken != nil {
		updates["bot_token"] = *botToken
	}
	if chatID != nil {
		updates["chat_id"] = *chatID
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	return s.repo.UpdateTelegramConfig(ctx, userAddress, *name, updates)
}

// updateLarkConfig 更新Lark配置
func (s *notificationService) updateLarkConfig(ctx context.Context, userAddress string, name *string, webhookURL *string, secret *string, isActive *bool) error {
	// 检查配置是否存在
	_, err := s.repo.GetLarkConfigByUserAddressAndName(ctx, userAddress, *name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("lark config not found")
		}
		return fmt.Errorf("failed to get lark config: %w", err)
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if webhookURL != nil {
		updates["webhook_url"] = *webhookURL
	}
	if secret != nil {
		updates["secret"] = *secret
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	return s.repo.UpdateLarkConfig(ctx, userAddress, *name, updates)
}

// updateFeishuConfig 更新Feishu配置
func (s *notificationService) updateFeishuConfig(ctx context.Context, userAddress string, name *string, webhookURL *string, secret *string, isActive *bool) error {
	// 检查配置是否存在
	_, err := s.repo.GetFeishuConfigByUserAddressAndName(ctx, userAddress, *name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("feishu config not found")
		}
		return fmt.Errorf("failed to get feishu config: %w", err)
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if webhookURL != nil {
		updates["webhook_url"] = *webhookURL
	}
	if secret != nil {
		updates["secret"] = *secret
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	return s.repo.UpdateFeishuConfig(ctx, userAddress, *name, updates)
}

// updateDiscordConfig 更新Discord配置
func (s *notificationService) updateDiscordConfig(ctx context.Context, userAddress string, name *string, webhookURL *string, isActive *bool) error {
	// 检查配置是否存在
	_, err := s.repo.GetDiscordConfigByUserAddressAndName(ctx, userAddress, *name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("discord config not found")
		}
		return fmt.Errorf("failed to get discord config: %w", err)
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if webhookURL != nil {
		updates["webhook_url"] = *webhookURL
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	return s.repo.UpdateDiscordConfig(ctx, userAddress, *name, updates)
}

// updateSlackConfig 更新Slack配置
func (s *notificationService) updateSlackConfig(ctx context.Context, userAddress string, name *string, webhookURL *string, isActive *bool) error {
	// 检查配置是否存在
	_, err := s.repo.GetSlackConfigByUserAddressAndName(ctx, userAddress, *name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("slack config not found")
		}
		return fmt.Errorf("failed to get slack config: %w", err)
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if webhookURL != nil {
		updates["webhook_url"] = *webhookURL
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	return s.repo.UpdateSlackConfig(ctx, userAddress, *name, updates)
}

// ===== 删除配置 =====
// deleteTelegramConfig 删除Telegram配置
func (s *notificationService) deleteTelegramConfig(ctx context.Context, userAddress string, name string) error {
	// 检查配置是否存在
	_, err := s.repo.GetTelegramConfigByUserAddressAndName(ctx, userAddress, name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("telegram config not found")
		}
		return fmt.Errorf("failed to get telegram config: %w", err)
	}

	return s.repo.DeleteTelegramConfig(ctx, userAddress, name)
}

// deleteLarkConfig 删除Lark配置
func (s *notificationService) deleteLarkConfig(ctx context.Context, userAddress string, name string) error {
	// 检查配置是否存在
	_, err := s.repo.GetLarkConfigByUserAddressAndName(ctx, userAddress, name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("lark config not found")
		}
		return fmt.Errorf("failed to get lark config: %w", err)
	}

	return s.repo.DeleteLarkConfig(ctx, userAddress, name)
}

// deleteFeishuConfig 删除Feishu配置
func (s *notificationService) deleteFeishuConfig(ctx context.Context, userAddress string, name string) error {
	// 检查配置是否存在
	_, err := s.repo.GetFeishuConfigByUserAddressAndName(ctx, userAddress, name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("feishu config not found")
		}
		return fmt.Errorf("failed to get feishu config: %w", err)
	}

	return s.repo.DeleteFeishuConfig(ctx, userAddress, name)
}

// deleteDiscordConfig 删除Discord配置
func (s *notificationService) deleteDiscordConfig(ctx context.Context, userAddress string, name string) error {
	// 检查配置是否存在
	_, err := s.repo.GetDiscordConfigByUserAddressAndName(ctx, userAddress, name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("discord config not found")
		}
		return fmt.Errorf("failed to get discord config: %w", err)
	}

	return s.repo.DeleteDiscordConfig(ctx, userAddress, name)
}

// deleteSlackConfig 删除Slack配置
func (s *notificationService) deleteSlackConfig(ctx context.Context, userAddress string, name string) error {
	// 检查配置是否存在
	_, err := s.repo.GetSlackConfigByUserAddressAndName(ctx, userAddress, name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("slack config not found")
		}
		return fmt.Errorf("failed to get slack config: %w", err)
	}

	return s.repo.DeleteSlackConfig(ctx, userAddress, name)
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

	// 获取Discord配置
	discordConfigs, err := s.repo.GetDiscordConfigsByUserAddress(ctx, userAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get discord configs: %w", err)
	}
	response.DiscordConfigs = discordConfigs

	// 获取Slack配置
	slackConfigs, err := s.repo.GetSlackConfigsByUserAddress(ctx, userAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get slack configs: %w", err)
	}
	response.SlackConfigs = slackConfigs

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

	var notificationData *types.NotificationData
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
		// 简化显示的交易哈希（前10位...后6位）
		if len(*txHash) > 10 {
			txDisplay = fmt.Sprintf("%s...%s", (*txHash)[:10], (*txHash)[len(*txHash)-6:])
		} else {
			txDisplay = *txHash
		}
	} else {
		txDisplay = "Pending"
		txLink = ""
	}

	if standard == "compound" {
		// 获取合约信息
		compoundTimeLock, err := s.timelockRepo.GetCompoundTimeLockByChainAndAddress(ctx, chainID, contractAddress)
		if err != nil {
			logger.Error("Failed to get compound time lock", err, "chainID", chainID, "contractAddress", contractAddress)
			return fmt.Errorf("failed to get compound timelock: %w", err)
		}

		// 从 Goldsky Flow 表中获取 Flow 信息
		flow, err := s.flowRepo.GetCompoundFlowByID(ctx, flowID, chainID, contractAddress)
		if err != nil {
			logger.Error("Failed to get compound flow", err, "flowID", flowID, "chainID", chainID, "contractAddress", contractAddress)
			return fmt.Errorf("failed to get compound flow: %w", err)
		}
		if flow == nil {
			logger.Warn("No compound flow found", "flowID", flowID, "chainID", chainID, "contractAddress", contractAddress)
			return nil
		}

		var functionName string
		var calldataParams []types.CalldataParam
		var caller string
		var target string

		// 获取 caller
		if flow.InitiatorAddress != nil {
			caller = *flow.InitiatorAddress
		} else if initiatorAddress != "" {
			caller = initiatorAddress
		} else {
			caller = "Unknown"
		}

		// 获取 target
		if flow.TargetAddress != nil {
			target = *flow.TargetAddress
		} else {
			target = "Unknown"
		}

		// 解析calldata
		if flow.CallData != nil && flow.FunctionSignature != nil {
			functionName = *flow.FunctionSignature
			calldataParams, err = utils.ParseCalldataNoSelector(*flow.FunctionSignature, flow.CallData)
			if err != nil {
				calldataParams = []types.CalldataParam{
					{
						Name:  "param[0]",
						Type:  "CallData Does Not Match Function Signature",
						Value: "Please Check Your Call Data",
					},
				}
				logger.Error("Failed to parse calldata", err, "functionSignature", *flow.FunctionSignature, "callData", flow.CallData)
			}
		} else {
			functionName = "No Function Call"
			calldataParams = []types.CalldataParam{}
		}

		nativeToken := chainInfo.NativeCurrencySymbol
		value, err := utils.WeiToEth(flow.Value, nativeToken)
		if err != nil {
			logger.Error("Failed to convert wei to eth", err, "eventValue", flow.Value)
			value = fmt.Sprintf("0 %s", nativeToken)
		}

		notificationData = &types.NotificationData{
			Standard:       strings.ToUpper(standard),
			Contract:       contractAddress,
			Remark:         compoundTimeLock.Remark,
			Caller:         caller,
			Target:         target,
			Function:       functionName,
			Value:          value,
			CalldataParams: calldataParams,
		}
	} else if standard == "openzeppelin" {
		// 拿合约信息
		// 通过flowID去交易表中拿到交易信息
		// 解析calldata(由于OZevent中直接是calldata带函数选择器，需要先识别functionSig，然后解析calldata)
		// （functionSig可以新建一个functionSig表，用于存储functionSig和functionName的映射，计算用户导入的abi里的函数，然后存储到functionSig表中）
		// 构建emailData
	} else {
		return fmt.Errorf("invalid standard")
	}

	notificationData.StatusFrom = strings.ToUpper(statusFrom)
	notificationData.StatusTo = strings.ToUpper(statusTo)
	notificationData.Network = chainInfo.DisplayName
	notificationData.TxHash = txDisplay
	notificationData.TxUrl = txLink
	notificationData.DashboardUrl = s.config.Email.EmailURL

	// 生成通知消息
	message, err := s.generateNotificationMessage(ctx, notificationData)
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
		totalConfigs := len(configs.TelegramConfigs) + len(configs.LarkConfigs) + len(configs.FeishuConfigs) + len(configs.DiscordConfigs) + len(configs.SlackConfigs)
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

		// 发送Discord通知
		for _, config := range configs.DiscordConfigs {
			s.sendDiscordNotification(ctx, config, message, flowID, standard, chainID, contractAddress, statusFrom, statusTo, txHash)
			totalSent++
		}

		// 发送Slack通知
		for _, config := range configs.SlackConfigs {
			s.sendSlackNotification(ctx, config, message, flowID, standard, chainID, contractAddress, statusFrom, statusTo, txHash)
			totalSent++
		}
	}

	logger.Info("Notification sending completed", "totalUsers", len(userAddresses), "totalNotificationsSent", totalSent)
	return nil
}

// generateNotificationMessage 生成通知消息
func (s *notificationService) generateNotificationMessage(ctx context.Context, notificationData *types.NotificationData) (string, error) {

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
	message += fmt.Sprintf("⚡ Timelock Notification\n")
	message += fmt.Sprintf("━━━━━━━━━━━━━━━━\n")
	message += fmt.Sprintf("[%s] %s    ➡️    [%s] %s\n", strings.ToUpper(notificationData.StatusFrom), getStatusEmoji(notificationData.StatusFrom), strings.ToUpper(notificationData.StatusTo), getStatusEmoji(notificationData.StatusTo))
	message += fmt.Sprintf("🔗 Chain    : %s\n", notificationData.Network)
	message += fmt.Sprintf("📄 Contract : %s\n", notificationData.Contract)
	message += fmt.Sprintf("⚙️ Standard : %s\n", strings.ToUpper(notificationData.Standard))
	message += fmt.Sprintf("💬 Remark   : %s\n", notificationData.Remark)
	message += fmt.Sprintf("👤 Caller   : %s\n", notificationData.Caller)
	message += fmt.Sprintf("🎯 Target   : %s\n", notificationData.Target)
	message += fmt.Sprintf("💰 Value    : %s\n", notificationData.Value)
	message += fmt.Sprintf("🔍 Function : %s\n", notificationData.Function)
	for _, param := range notificationData.CalldataParams {
		message += fmt.Sprintf("    🔒 %s(%s) : %s\n", param.Name, param.Type, param.Value)
	}
	message += fmt.Sprintf("🔍 Tx Hash  : %s\n", notificationData.TxHash)
	message += fmt.Sprintf("🔗 Tx URL  : %s\n", notificationData.TxUrl)

	logger.Info("Generated notification message", "statusFrom", notificationData.StatusFrom, "statusTo", notificationData.StatusTo, "txHash", notificationData.TxHash)
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

// sendDiscordNotification 发送Discord通知
func (s *notificationService) sendDiscordNotification(ctx context.Context, config *types.DiscordConfig, message, flowID, standard string, chainID int, contractAddress, statusFrom, statusTo string, txHash *string) {
	// 检查是否已发送过此通知
	exists, err := s.repo.CheckNotificationLogExists(ctx, types.ChannelDiscord, config.UserAddress, config.ID, flowID, statusTo)
	if err != nil {
		logger.Error("Failed to check discord notification log", err, "configID", config.ID, "flowID", flowID)
		return
	}
	if exists {
		logger.Info("Discord notification already sent", "configID", config.ID, "flowID", flowID, "status", statusTo)
		return
	}

	// 发送消息
	err = s.discordSender.SendMessage(config.WebhookURL, message)
	sendStatus := "success"
	var errorMessage *string
	if err != nil {
		sendStatus = "failed"
		errMsg := err.Error()
		errorMessage = &errMsg
		logger.Error("Failed to send discord notification", err, "configID", config.ID, "flowID", flowID)
	}

	// 记录发送日志
	log := &types.NotificationLog{
		UserAddress:      config.UserAddress,
		Channel:          types.ChannelDiscord,
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
		logger.Error("Failed to create discord notification log", err, "configID", config.ID, "flowID", flowID)
	}

	if sendStatus == "success" {
		logger.Info("Discord notification sent", "configID", config.ID, "flowID", flowID, "status", statusTo)
	}
}

// sendSlackNotification 发送Slack通知
func (s *notificationService) sendSlackNotification(ctx context.Context, config *types.SlackConfig, message, flowID, standard string, chainID int, contractAddress, statusFrom, statusTo string, txHash *string) {
	// 检查是否已发送过此通知
	exists, err := s.repo.CheckNotificationLogExists(ctx, types.ChannelSlack, config.UserAddress, config.ID, flowID, statusTo)
	if err != nil {
		logger.Error("Failed to check slack notification log", err, "configID", config.ID, "flowID", flowID)
		return
	}
	if exists {
		logger.Info("Slack notification already sent", "configID", config.ID, "flowID", flowID, "status", statusTo)
		return
	}

	// 发送消息
	err = s.slackSender.SendMessage(config.WebhookURL, message)
	sendStatus := "success"
	var errorMessage *string
	if err != nil {
		sendStatus = "failed"
		errMsg := err.Error()
		errorMessage = &errMsg
		logger.Error("Failed to send slack notification", err, "configID", config.ID, "flowID", flowID)
	}

	// 记录发送日志
	log := &types.NotificationLog{
		UserAddress:      config.UserAddress,
		Channel:          types.ChannelSlack,
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
		logger.Error("Failed to create slack notification log", err, "configID", config.ID, "flowID", flowID)
	}

	if sendStatus == "success" {
		logger.Info("Slack notification sent", "configID", config.ID, "flowID", flowID, "status", statusTo)
	}
}
