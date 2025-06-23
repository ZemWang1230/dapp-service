package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"
	"timelocker-backend/internal/config"
	"timelocker-backend/pkg/logger"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// RPCClient 区块链RPC客户端接口
type RPCClient interface {
	GetNativeBalance(ctx context.Context, chainID int64, address string) (*big.Int, error)
	GetTokenBalance(ctx context.Context, chainID int64, address, contractAddress string) (*big.Int, error)
	Close()
}

// rpcClient RPC客户端实现
type rpcClient struct {
	config  *config.RPCConfig
	clients map[int64]*ethclient.Client
}

// NewRPCClient 创建新的RPC客户端
func NewRPCClient(cfg *config.RPCConfig) (RPCClient, error) {
	client := &rpcClient{
		config:  cfg,
		clients: make(map[int64]*ethclient.Client),
	}

	// 预连接支持的链
	chainConfigs := map[int64]string{
		1:     "ethereum", // Ethereum Mainnet
		56:    "bsc",      // BSC Mainnet
		137:   "polygon",  // Polygon Mainnet
		42161: "arbitrum", // Arbitrum One
	}

	for chainID, chainKey := range chainConfigs {
		if err := client.connectChain(chainID, chainKey); err != nil {
			logger.Error("Failed to connect to chain", err, "chain_id", chainID, "chain_key", chainKey)
			// 不返回错误，继续连接其他链
		}
	}

	return client, nil
}

// connectChain 连接指定链
func (c *rpcClient) connectChain(chainID int64, chainKey string) error {
	rpcURL := c.getRPCURL(chainKey)
	if rpcURL == "" {
		logger.Error("connectChain", fmt.Errorf("RPC URL not configured for chain: %s", chainKey))
		return fmt.Errorf("RPC URL not configured for chain: %s", chainKey)
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		logger.Error("connectChain", err, "chain_id", chainID, "chain_key", chainKey)
		return fmt.Errorf("failed to connect to %s: %w", chainKey, err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.ChainID(ctx)
	if err != nil {
		client.Close()
		logger.Error("connectChain", err, "chain_id", chainID, "chain_key", chainKey)
		return fmt.Errorf("failed to get chain ID for %s: %w", chainKey, err)
	}

	c.clients[chainID] = client
	logger.Info("Successfully connected to chain", "chain_id", chainID, "chain_key", chainKey, "rpc_url", c.maskURL(rpcURL))
	return nil
}

// getRPCURL 获取RPC URL
func (c *rpcClient) getRPCURL(chainKey string) string {
	var baseURL, apiKey string

	switch strings.ToLower(c.config.Provider) {
	case "alchemy":
		apiKey = c.config.Alchemy.APIKey
		switch chainKey {
		case "ethereum":
			baseURL = c.config.Alchemy.Ethereum
		case "bsc":
			baseURL = c.config.Alchemy.BSC
		case "polygon":
			baseURL = c.config.Alchemy.Polygon
		case "arbitrum":
			baseURL = c.config.Alchemy.Arbitrum
		}
	case "infura":
		apiKey = c.config.Infura.APIKey
		switch chainKey {
		case "ethereum":
			baseURL = c.config.Infura.Ethereum
		case "bsc":
			baseURL = c.config.Infura.BSC
		case "polygon":
			baseURL = c.config.Infura.Polygon
		case "arbitrum":
			baseURL = c.config.Infura.Arbitrum
		}
	}

	if baseURL == "" || apiKey == "" {
		logger.Error("getRPCURL", fmt.Errorf("RPC URL or API key not configured for chain: %s", chainKey))
		return ""
	}

	// 组合URL
	if strings.HasSuffix(baseURL, "/") {
		return baseURL + apiKey
	}
	return baseURL + "/" + apiKey
}

// maskURL 遮蔽URL中的API密钥用于日志记录
func (c *rpcClient) maskURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if len(lastPart) > 8 {
			parts[len(parts)-1] = lastPart[:4] + "****" + lastPart[len(lastPart)-4:]
		}
	}
	return strings.Join(parts, "/")
}

// getClient 获取指定链的客户端
func (c *rpcClient) getClient(chainID int64) *ethclient.Client {
	return c.clients[chainID]
}

// GetNativeBalance 获取原生代币余额
func (c *rpcClient) GetNativeBalance(ctx context.Context, chainID int64, address string) (*big.Int, error) {
	client := c.getClient(chainID)
	if client == nil {
		logger.Error("getClient", fmt.Errorf("no client available for chain ID: %d", chainID))
		return nil, fmt.Errorf("no client available for chain ID: %d", chainID)
	}

	// 验证地址格式
	if !common.IsHexAddress(address) {
		logger.Error("GetNativeBalance", fmt.Errorf("invalid address format: %s", address))
		return nil, fmt.Errorf("invalid address format: %s", address)
	}

	addr := common.HexToAddress(address)
	balance, err := client.BalanceAt(ctx, addr, nil)
	if err != nil {
		logger.Error("GetNativeBalance", err, "chain_id", chainID, "address", address)
		return nil, fmt.Errorf("failed to get native balance: %w", err)
	}

	logger.Info("Got native balance", "chain_id", chainID, "address", address, "balance", balance.String())
	return balance, nil
}

// GetTokenBalance 获取ERC-20代币余额
func (c *rpcClient) GetTokenBalance(ctx context.Context, chainID int64, address, contractAddress string) (*big.Int, error) {
	client := c.getClient(chainID)
	if client == nil {
		logger.Error("getClient", fmt.Errorf("no client available for chain ID: %d", chainID))
		return nil, fmt.Errorf("no client available for chain ID: %d", chainID)
	}

	// 验证地址格式
	if !common.IsHexAddress(address) {
		logger.Error("GetTokenBalance", fmt.Errorf("invalid address format: %s", address))
		return nil, fmt.Errorf("invalid address format: %s", address)
	}
	if !common.IsHexAddress(contractAddress) {
		logger.Error("GetTokenBalance", fmt.Errorf("invalid contract address format: %s", contractAddress))
		return nil, fmt.Errorf("invalid contract address format: %s", contractAddress)
	}

	// ERC-20 balanceOf 方法的ABI编码
	// balanceOf(address) -> 0x70a08231
	data := make([]byte, 36)
	copy(data[:4], []byte{0x70, 0xa0, 0x82, 0x31})        // 方法签名
	copy(data[16:], common.HexToAddress(address).Bytes()) // 地址参数，左侧补零到32字节

	msg := ethereum.CallMsg{
		To:   &common.Address{},
		Data: data,
	}
	copy(msg.To[:], common.HexToAddress(contractAddress).Bytes())

	result, err := client.CallContract(ctx, msg, nil)
	if err != nil {
		logger.Error("Failed to call token contract", err, "chain_id", chainID, "address", address, "contract", contractAddress)
		return nil, fmt.Errorf("failed to call token contract: %w", err)
	}

	// 解析返回值
	if len(result) != 32 {
		logger.Error("GetTokenBalance", fmt.Errorf("unexpected result length: %d", len(result)))
		return nil, fmt.Errorf("unexpected result length: %d", len(result))
	}

	balance := new(big.Int).SetBytes(result)
	logger.Info("Got token balance", "chain_id", chainID, "address", address, "contract", contractAddress, "balance", balance.String())
	return balance, nil
}

// Close 关闭所有连接
func (c *rpcClient) Close() {
	for chainID, client := range c.clients {
		client.Close()
		logger.Info("Closed RPC client", "chain_id", chainID)
	}
	c.clients = make(map[int64]*ethclient.Client)
}
