package types

import (
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

// RPCNode RPC节点信息
type RPCNode struct {
	URL      string
	ChainID  int
	Client   *ethclient.Client
	Metadata *RPCMetadata
}

// RPCMetadata RPC元数据
type RPCMetadata struct {
	URL            string    `json:"url"`
	ChainID        int       `json:"chain_id"`
	IsHealthy      bool      `json:"is_healthy"`
	MaxSafeRange   int       `json:"max_safe_range"`
	LastCheckedAt  time.Time `json:"last_checked_at"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	ErrorCount     int       `json:"error_count"`
	LastError      *string   `json:"last_error"`
}

// ChainlistResponse chainlist.org API响应结构
type ChainlistResponse []ChainInfo

// ChainInfo 链信息
type ChainInfo struct {
	ChainID        int            `json:"chainId"`
	NetworkID      int            `json:"networkId"`
	NativeCurrency NativeCurrency `json:"nativeCurrency"`
	RPC            []RPCEndpoint  `json:"rpc"`
	Explorers      []Explorer     `json:"explorers"`
}

// NativeCurrency 原生货币信息
type NativeCurrency struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

// RPCEndpoint RPC端点信息
type RPCEndpoint struct {
	URL          string `json:"url"`
	Tracking     string `json:"tracking"`
	IsOpenSource bool   `json:"isOpenSource"`
}

// Explorer 区块浏览器信息
type Explorer struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Standard string `json:"standard"`
}

// HealthCheckResult 健康检查和能力测试统一结果
type HealthCheckResult struct {
	URL          string        `json:"url"`
	ChainID      int           `json:"chain_id"`
	IsHealthy    bool          `json:"is_healthy"`
	MaxSafeRange int           `json:"max_safe_range"` // FilterLogs最大安全范围
	ResponseTime time.Duration `json:"response_time"`
	Error        string        `json:"error,omitempty"`
	CheckedAt    time.Time     `json:"checked_at"`
}

// RPCPoolStatus RPC池状态
type RPCPoolStatus struct {
	ChainID     int             `json:"chain_id"`
	TotalRPCs   int             `json:"total_rpcs"`
	HealthyRPCs int             `json:"healthy_rpcs"`
	RPCs        []RPCNodeStatus `json:"rpcs"`
	LastUpdated time.Time       `json:"last_updated"`
}

// RPCNodeStatus RPC节点状态
type RPCNodeStatus struct {
	URL            string    `json:"url"`
	IsHealthy      bool      `json:"is_healthy"`
	MaxSafeRange   int       `json:"max_safe_range"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	ErrorCount     int       `json:"error_count"`
	LastError      *string   `json:"last_error"`
	LastCheckedAt  time.Time `json:"last_checked_at"`
}
