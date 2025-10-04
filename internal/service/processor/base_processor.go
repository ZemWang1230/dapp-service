package processor

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
)

// RPCManager 接口定义
type RPCManager interface {
	ExecuteWithRPCInfoDo(ctx context.Context, chainID int, fn func(*ethclient.Client, int) error) (string, int, error)
	ExecuteWithRPCInfoDoInfiniteRetry(ctx context.Context, chainID int, fn func(*ethclient.Client, int) error) (string, int, error)
}

// BaseProcessor 基础处理器（提供通用功能）
type BaseProcessor struct {
	chainID    int
	chainName  string
	rpcManager RPCManager
}

// NewBaseProcessor 创建基础处理器
func NewBaseProcessor(chainID int, chainName string) *BaseProcessor {
	return &BaseProcessor{
		chainID:   chainID,
		chainName: chainName,
	}
}

// SetRPCManager 设置RPC管理器
func (bp *BaseProcessor) SetRPCManager(rpcManager RPCManager) {
	bp.rpcManager = rpcManager
}

// GetChainID 获取链ID
func (bp *BaseProcessor) GetChainID() int {
	return bp.chainID
}

// GetChainName 获取链名称
func (bp *BaseProcessor) GetChainName() string {
	return bp.chainName
}

// SetChainInfo 设置链ID和链名称
func (bp *BaseProcessor) SetChainInfo(chainID int, chainName string) {
	bp.chainID = chainID
	bp.chainName = chainName
}
