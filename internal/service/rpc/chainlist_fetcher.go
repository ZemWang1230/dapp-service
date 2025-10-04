package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// ChainlistFetcher chainlist.org数据获取器
type ChainlistFetcher struct {
	httpClient *http.Client
	apiURL     string
}

// NewChainlistFetcher 创建chainlist数据获取器
func NewChainlistFetcher() *ChainlistFetcher {
	return &ChainlistFetcher{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiURL: "https://chainlist.org/rpcs.json",
	}
}

// FetchChainData 获取指定链ID的链数据
func (cf *ChainlistFetcher) FetchChainData(ctx context.Context, chainID int) (*types.ChainInfo, error) {
	logger.Info("Fetching chain data from chainlist.org", "chain_id", chainID)

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", cf.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "TimeLock/1.0")
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := cf.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chainlist data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chainlist API returned status %d", resp.StatusCode)
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 解析JSON
	var chains types.ChainlistResponse
	if err := json.Unmarshal(body, &chains); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chainlist response: %w", err)
	}

	// 查找指定链ID的数据
	for _, chain := range chains {
		if chain.ChainID == chainID {
			logger.Info("Found chain data", "chain_id", chainID, "rpc_count", len(chain.RPC))
			return &chain, nil
		}
	}

	return nil, fmt.Errorf("chain ID %d not found in chainlist", chainID)
}

// FetchMultipleChainData 批量获取多个链的数据
func (cf *ChainlistFetcher) FetchMultipleChainData(ctx context.Context, chainIDs []int) (map[int]*types.ChainInfo, error) {
	logger.Info("Fetching multiple chain data from chainlist.org", "chain_count", len(chainIDs))

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", cf.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "TimeLock/1.0")
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := cf.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chainlist data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chainlist API returned status %d", resp.StatusCode)
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 解析JSON
	var chains types.ChainlistResponse
	if err := json.Unmarshal(body, &chains); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chainlist response: %w", err)
	}

	// 创建链ID映射
	chainIDMap := make(map[int]bool)
	for _, id := range chainIDs {
		chainIDMap[id] = true
	}

	// 查找所有匹配的链数据
	result := make(map[int]*types.ChainInfo)
	for _, chain := range chains {
		if chainIDMap[chain.ChainID] {
			result[chain.ChainID] = &chain
		}
	}

	logger.Info("Found chain data", "requested", len(chainIDs), "found", len(result))
	return result, nil
}

// FilterHTTPSRPCs 过滤出HTTPS协议的RPC URLs
func (cf *ChainlistFetcher) FilterHTTPSRPCs(rpcs []types.RPCEndpoint) []string {
	var httpsRPCs []string
	var strongRPCs []string
	// 1.https的
	// 2.不包含变量的RPC URL
	for _, rpc := range rpcs {
		if strings.HasPrefix(strings.ToLower(rpc.URL), "https://") {
			// 跳过包含变量的RPC URL (如 ${API_KEY})
			if !strings.Contains(rpc.URL, "${") && !strings.Contains(rpc.URL, "{") {
				httpsRPCs = append(httpsRPCs, rpc.URL)
				if rpc.Tracking == "none" {
					strongRPCs = append(strongRPCs, rpc.URL)
				}
			}
		}
	}

	if len(strongRPCs) > 0 {
		return strongRPCs
	}
	return httpsRPCs
}

// GetFirstExplorerURL 获取第一个区块浏览器URL
func (cf *ChainlistFetcher) GetFirstExplorerURL(explorers []types.Explorer) string {
	for _, explorer := range explorers {
		if explorer.URL != "" {
			return explorer.URL
		}
	}
	return ""
}

// ValidateChainData 验证链数据的完整性
func (cf *ChainlistFetcher) ValidateChainData(chain *types.ChainInfo) error {
	if chain.ChainID <= 0 {
		return fmt.Errorf("invalid chain ID: %d", chain.ChainID)
	}

	if chain.NativeCurrency.Name == "" {
		return fmt.Errorf("missing native currency name for chain %d", chain.ChainID)
	}

	if chain.NativeCurrency.Symbol == "" {
		return fmt.Errorf("missing native currency symbol for chain %d", chain.ChainID)
	}

	if chain.NativeCurrency.Decimals <= 0 {
		return fmt.Errorf("invalid native currency decimals for chain %d: %d", chain.ChainID, chain.NativeCurrency.Decimals)
	}

	httpsRPCs := cf.FilterHTTPSRPCs(chain.RPC)
	if len(httpsRPCs) == 0 {
		return fmt.Errorf("no valid HTTPS RPC URLs found for chain %d", chain.ChainID)
	}

	return nil
}
