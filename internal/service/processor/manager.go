package processor

import (
	"context"
	"fmt"

	"timelocker-backend/internal/types"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ChainLogProcessor 链日志处理器接口
type ChainLogProcessor interface {
	// ProcessLog 处理单个日志事件，返回TimelockEvent
	ProcessLog(ctx context.Context, client *ethclient.Client, log *ethtypes.Log) (types.TimelockEvent, error)

	// GetChainID 获取支持的链ID
	GetChainID() int

	// GetChainName 获取链名称
	GetChainName() string
}

// ProcessorManager 处理器管理器
type ProcessorManager struct {
	processors map[int]ChainLogProcessor
	rpcManager RPCManager
}

// NewProcessorManagerWithRPCManager 创建带RPC管理器的处理器管理器
func NewProcessorManagerWithRPCManager(rpcManager RPCManager) *ProcessorManager {
	pm := &ProcessorManager{
		processors: make(map[int]ChainLogProcessor),
		rpcManager: rpcManager,
	}

	// 注册带RPC管理器的处理器
	pm.registerDefaultProcessorsWithRPCManager()

	return pm
}

// registerDefaultProcessorsWithRPCManager 注册带RPC管理器的默认处理器
func (pm *ProcessorManager) registerDefaultProcessorsWithRPCManager() {
	// 为每条链创建独立处理器实例，并在创建时设置链信息，避免共享实例导致链信息不一致
	// EVM系列
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(1, "ethereum-mainnet")
		pm.RegisterProcessor(1, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(56, "bsc-mainnet")
		pm.RegisterProcessor(56, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(1116, "core-mainnet")
		pm.RegisterProcessor(1116, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(196, "xlayer-mainnet")
		pm.RegisterProcessor(196, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(59144, "linea-mainnet")
		pm.RegisterProcessor(59144, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(534352, "scroll-mainnet")
		pm.RegisterProcessor(534352, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(223, "b2-mainnet")
		pm.RegisterProcessor(223, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(43111, "hemi-mainnet")
		pm.RegisterProcessor(43111, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(2345, "goat-mainnet")
		pm.RegisterProcessor(2345, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(200901, "bitlayer-mainnet")
		pm.RegisterProcessor(200901, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(98866, "plume-mainnet")
		pm.RegisterProcessor(98866, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(177, "hashkey-mainnet")
		pm.RegisterProcessor(177, p)
	}
	// 待测试的链
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(4200, "merlin-mainnet")
		pm.RegisterProcessor(4200, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(2649, "ailayer-mainnet")
		pm.RegisterProcessor(2649, p)
	}

	// 测试网
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(11155111, "sepolia-testnet")
		pm.RegisterProcessor(11155111, p)
	}
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(97, "bsc-testnet")
		pm.RegisterProcessor(97, p)
	}

	// L2链
	{
		p := NewL2ProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(42161, "arbitrum-mainnet")
		pm.RegisterProcessor(42161, p)
	}
	{
		p := NewL2ProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(10, "optimism-mainnet")
		pm.RegisterProcessor(10, p)
	}
	{
		p := NewL2ProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(8453, "base-mainnet")
		pm.RegisterProcessor(8453, p)
	}
	{
		p := NewL2ProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(34443, "mode-mainnet")
		pm.RegisterProcessor(34443, p)
	}
	// 非严格L2但复用EVM处理器
	{
		p := NewEthereumProcessorWithRPCManager(pm.rpcManager)
		p.SetChainInfo(810180, "zklink-mainnet")
		pm.RegisterProcessor(810180, p)
	}
}

// RegisterProcessor 注册处理器
func (pm *ProcessorManager) RegisterProcessor(chainID int, processor ChainLogProcessor) {
	pm.processors[chainID] = processor
}

// GetProcessor 获取指定链的处理器
func (pm *ProcessorManager) GetProcessor(chainID int) (ChainLogProcessor, error) {
	processor, exists := pm.processors[chainID]
	if !exists {
		return nil, fmt.Errorf("no processor found for chain %d", chainID)
	}
	return processor, nil
}

// ProcessLog 处理日志（带重试机制）
func (pm *ProcessorManager) ProcessLog(ctx context.Context, client *ethclient.Client, log *ethtypes.Log, chainID int) (types.TimelockEvent, error) {
	processor, err := pm.GetProcessor(chainID)
	if err != nil {
		return nil, err
	}

	// 使用处理器处理日志
	return processor.ProcessLog(ctx, client, log)
}
