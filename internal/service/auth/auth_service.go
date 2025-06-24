package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"timelocker-backend/internal/repository/user"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/crypto"
	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/utils"

	"gorm.io/gorm"
)

var (
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrInvalidAddress    = errors.New("invalid wallet address")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidToken      = errors.New("invalid token")
	ErrTokenExpired      = errors.New("token expired")
	ErrSignatureRecovery = errors.New("failed to recover address from signature")
)

// AssetService 资产服务接口（避免循环依赖）
type AssetService interface {
	RefreshUserAssetsOnChainConnect(walletAddress string) error
}

// Service 认证服务接口
type Service interface {
	WalletConnect(ctx context.Context, req *types.WalletConnectRequest) (*types.WalletConnectResponse, error)
	RefreshToken(ctx context.Context, req *types.RefreshTokenRequest) (*types.WalletConnectResponse, error)
	GetProfile(ctx context.Context, walletAddress string) (*types.UserProfile, error)
	VerifyToken(ctx context.Context, tokenString string) (*types.JWTClaims, error)
	SwitchChain(ctx context.Context, walletAddress string, req *types.SwitchChainRequest) (*types.SwitchChainResponse, error)
	SetAssetService(assetService AssetService)
}

type service struct {
	userRepo     user.Repository
	jwtManager   *utils.JWTManager
	assetService AssetService
}

func NewService(userRepo user.Repository, jwtManager *utils.JWTManager) Service {
	return &service{
		userRepo:   userRepo,
		jwtManager: jwtManager,
	}
}

// SetAssetService 设置资产服务（避免循环依赖）
func (s *service) SetAssetService(assetService AssetService) {
	s.assetService = assetService
}

// WalletConnect 处理钱包连接认证
// 1. 验证钱包地址格式
// 2. 标准化地址格式
// 3. 验证签名
// 4. 查找或创建用户，更新最后登录的链ID
// 5. 生成JWT令牌
// 6. 触发资产更新
// 7. 返回认证响应
func (s *service) WalletConnect(ctx context.Context, req *types.WalletConnectRequest) (*types.WalletConnectResponse, error) {
	// 1. 验证钱包地址格式
	if !crypto.ValidateEthereumAddress(req.WalletAddress) {
		logger.Error("WalletConnect Error: ", ErrInvalidAddress)
		return nil, ErrInvalidAddress
	}

	// 2. 标准化地址格式
	normalizedAddress := crypto.NormalizeAddress(req.WalletAddress)

	// 3. 验证签名
	err := crypto.VerifySignature(req.Message, req.Signature, req.WalletAddress)
	if err != nil {
		// 尝试从签名中恢复地址进行二次验证
		recoveredAddress, recoverErr := crypto.RecoverAddress(req.Message, req.Signature)
		if recoverErr != nil {
			logger.Error("WalletConnect Error: ", ErrSignatureRecovery, recoverErr)
			return nil, fmt.Errorf("%w: %v", ErrSignatureRecovery, recoverErr)
		}

		// 验证恢复的地址是否与标准化后的地址一致
		if strings.ToLower(recoveredAddress) != normalizedAddress {
			logger.Error("WalletConnect Error: ", ErrInvalidSignature)
			return nil, fmt.Errorf("%w: signature does not match wallet address", ErrInvalidSignature)
		}
	}

	// 4. 查找或创建用户（以钱包地址为核心）
	existingUser, err := s.userRepo.GetUserByWallet(ctx, normalizedAddress)
	var currentUser *types.User

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新用户
			newUser := &types.User{
				WalletAddress: normalizedAddress,
				ChainID:       req.ChainId,
				Status:        1, // 1: 正常 0: 禁用
				Preferences:   make(types.JSONB),
			}

			if err := s.userRepo.CreateUser(ctx, newUser); err != nil {
				logger.Error("WalletConnect Error: ", errors.New("failed to create user"), "error: ", err)
				return nil, fmt.Errorf("failed to create user: %w", err)
			}
			currentUser = newUser
			logger.Info("WalletConnect: created new user", "wallet_address", normalizedAddress, "chain_id", req.ChainId, "user_id", newUser.ID)
		} else {
			// 数据库错误
			logger.Error("WalletConnect Error: ", errors.New("database error"), "error: ", err)
			return nil, fmt.Errorf("database error: %w", err)
		}
	} else {
		// 找到现有用户，更新最后登录时间和链ID
		if err := s.userRepo.UpdateLastLogin(ctx, normalizedAddress); err != nil {
			// 登录时间更新失败不应该阻止认证流程
			logger.Error("WalletConnect Error: ", errors.New("failed to update last login"), "error: ", err)
		}
		if err := s.userRepo.UpdateUserChainID(ctx, normalizedAddress, req.ChainId); err != nil {
			logger.Error("WalletConnect Error: ", errors.New("failed to update chain id"), "error: ", err)
		}
		// 更新用户信息
		existingUser.ChainID = req.ChainId
		currentUser = existingUser
		logger.Info("WalletConnect: found existing user", "wallet_address", normalizedAddress, "chain_id", req.ChainId, "user_id", existingUser.ID)
	}

	// 5. 生成JWT令牌
	accessToken, refreshToken, expiresAt, err := s.jwtManager.GenerateTokens(
		currentUser.ID,
		currentUser.WalletAddress,
	)
	if err != nil {
		logger.Error("WalletConnect Error: ", errors.New("failed to generate jwt tokens"), "error: ", err)
		return nil, fmt.Errorf("failed to generate jwt tokens: %w", err)
	}

	// 6. 异步触发资产更新（不阻塞认证流程）
	if s.assetService != nil {
		go func() {
			if err := s.assetService.RefreshUserAssetsOnChainConnect(normalizedAddress); err != nil {
				logger.Error("WalletConnect: failed to refresh user assets on chain connect", err, "wallet_address", normalizedAddress)
			} else {
				logger.Info("WalletConnect: successfully refreshed user assets on chain connect", "wallet_address", normalizedAddress)
			}
		}()
	}

	logger.Info("WalletConnect Response:", "User: ", currentUser.WalletAddress, "ChainID: ", currentUser.ChainID)
	// 7. 返回认证响应
	return &types.WalletConnectResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         *currentUser,
	}, nil
}

// RefreshToken 刷新访问令牌
// 1. 验证刷新令牌
// 2. 获取用户信息
// 3. 验证用户状态
// 4. 生成新的令牌对
// 5. 更新最后登录时间
// 6. 返回认证响应
func (s *service) RefreshToken(ctx context.Context, req *types.RefreshTokenRequest) (*types.WalletConnectResponse, error) {
	// 1. 验证刷新令牌
	claims, err := s.jwtManager.VerifyRefreshToken(req.RefreshToken)
	if err != nil {
		logger.Error("RefreshToken Error: ", errors.New("failed to verify refresh token"), "error: ", err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// 2. 获取用户信息
	user, err := s.userRepo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("RefreshToken Error: ", ErrUserNotFound)
			return nil, ErrUserNotFound
		}
		logger.Error("RefreshToken Error: ", errors.New("database error"), "error: ", err)
		return nil, fmt.Errorf("database error: %w", err)
	}

	// 3. 验证用户状态
	if user.Status != 1 {
		logger.Error("RefreshToken Error: ", errors.New("user account is disabled"))
		return nil, errors.New("user account is disabled")
	}

	// 4. 生成新的令牌对
	accessToken, refreshToken, expiresAt, err := s.jwtManager.GenerateTokens(
		user.ID,
		user.WalletAddress,
	)
	if err != nil {
		logger.Error("RefreshToken Error: ", errors.New("failed to generate jwt tokens"), "error: ", err)
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// 5. 更新最后登录时间
	if err := s.userRepo.UpdateLastLogin(ctx, user.WalletAddress); err != nil {
		// 登录时间更新失败不应该阻止刷新流程
		logger.Error("RefreshToken Error: ", errors.New("failed to update last login"), "error: ", err)
	}

	logger.Info("RefreshToken Response:", "User: ", user.WalletAddress, "ChainID: ", user.ChainID)
	return &types.WalletConnectResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         *user,
	}, nil
}

// GetProfile 获取用户资料
func (s *service) GetProfile(ctx context.Context, walletAddress string) (*types.UserProfile, error) {
	user, err := s.userRepo.GetUserByWallet(ctx, walletAddress)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("GetProfile Error: ", ErrUserNotFound)
			return nil, ErrUserNotFound
		}
		logger.Error("GetProfile Error: ", errors.New("database error"), "error: ", err)
		return nil, fmt.Errorf("database error: %w", err)
	}

	logger.Info("GetProfile :", "User: ", user.WalletAddress)
	return &types.UserProfile{
		WalletAddress: user.WalletAddress,
		ChainID:       user.ChainID,
		CreatedAt:     user.CreatedAt,
		LastLogin:     user.LastLogin,
		Preferences:   user.Preferences,
	}, nil
}

// VerifyToken 验证访问令牌
func (s *service) VerifyToken(ctx context.Context, tokenString string) (*types.JWTClaims, error) {
	claims, err := s.jwtManager.VerifyAccessToken(tokenString)
	if err != nil {
		logger.Error("VerifyToken Error: ", errors.New("failed to verify access token"), "error: ", err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// 验证用户是否存在且有效
	user, err := s.userRepo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("VerifyToken Error: ", ErrUserNotFound)
			return nil, ErrUserNotFound
		}
		logger.Error("VerifyToken Error: ", errors.New("database error"), "error: ", err)
		return nil, fmt.Errorf("database error: %w", err)
	}

	if user.Status != 1 {
		logger.Error("VerifyToken Error: ", errors.New("user account is disabled"))
		return nil, errors.New("user account is disabled")
	}

	return claims, nil
}

// SwitchChain 处理链切换认证（需要重新签名）
// 1. 验证当前用户是否存在
// 2. 验证用户状态
// 3. 验证签名
// 4. 更新用户链ID
// 5. 生成新的JWT令牌
// 6. 触发资产更新
// 7. 返回认证响应
func (s *service) SwitchChain(ctx context.Context, walletAddress string, req *types.SwitchChainRequest) (*types.SwitchChainResponse, error) {
	// 1. 验证当前用户是否存在
	currentUser, err := s.userRepo.GetUserByWallet(ctx, walletAddress)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("SwitchChain Error: ", ErrUserNotFound)
			return nil, ErrUserNotFound
		}
		logger.Error("SwitchChain Error: ", errors.New("database error"), "error: ", err)
		return nil, fmt.Errorf("database error: %w", err)
	}

	// 2. 验证用户状态
	if currentUser.Status != 1 {
		logger.Error("SwitchChain Error: ", errors.New("user account is disabled"))
		return nil, errors.New("user account is disabled")
	}

	// 3. 验证签名
	err = crypto.VerifySignature(req.Message, req.Signature, walletAddress)
	if err != nil {
		// 尝试从签名中恢复地址进行二次验证
		recoveredAddress, recoverErr := crypto.RecoverAddress(req.Message, req.Signature)
		if recoverErr != nil {
			logger.Error("SwitchChain Error: ", ErrSignatureRecovery, recoverErr)
			return nil, fmt.Errorf("%w: %v", ErrSignatureRecovery, recoverErr)
		}

		// 验证恢复的地址是否与钱包地址一致
		if strings.ToLower(recoveredAddress) != strings.ToLower(walletAddress) {
			logger.Error("SwitchChain Error: ", ErrInvalidSignature)
			return nil, fmt.Errorf("%w: signature does not match wallet address", ErrInvalidSignature)
		}
	}

	// 4. 更新用户链ID和最后登录时间
	if err := s.userRepo.UpdateUserChainID(ctx, walletAddress, req.ChainID); err != nil {
		logger.Error("SwitchChain Error: ", errors.New("failed to update chain id"), "error: ", err)
		return nil, fmt.Errorf("failed to update chain id: %w", err)
	}
	if err := s.userRepo.UpdateLastLogin(ctx, walletAddress); err != nil {
		logger.Error("SwitchChain Error: ", errors.New("failed to update last login"), "error: ", err)
	}

	// 更新用户信息
	currentUser.ChainID = req.ChainID

	// 5. 生成新的JWT令牌
	accessToken, refreshToken, expiresAt, err := s.jwtManager.GenerateTokens(
		currentUser.ID,
		currentUser.WalletAddress,
	)
	if err != nil {
		logger.Error("SwitchChain Error: ", errors.New("failed to generate jwt tokens"), "error: ", err)
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// 6. 异步触发资产更新（不阻塞切换流程）
	if s.assetService != nil {
		go func() {
			if err := s.assetService.RefreshUserAssetsOnChainConnect(walletAddress); err != nil {
				logger.Error("SwitchChain: failed to refresh user assets on chain switch", err, "wallet_address", walletAddress, "chain_id", req.ChainID)
			} else {
				logger.Info("SwitchChain: successfully refreshed user assets on chain switch", "wallet_address", walletAddress, "chain_id", req.ChainID)
			}
		}()
	}

	message := fmt.Sprintf("Successfully switched to chain %d", req.ChainID)

	logger.Info("SwitchChain Response:", "User: ", currentUser.WalletAddress, "New ChainID: ", currentUser.ChainID)
	// 7. 返回认证响应
	return &types.SwitchChainResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         *currentUser,
		Message:      message,
	}, nil
}
