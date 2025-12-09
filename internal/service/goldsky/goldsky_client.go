package goldsky

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// GoldskyClient Goldsky GraphQL 客户端
type GoldskyClient struct {
	httpClient  *http.Client
	subgraphURL string
	chainID     int
}

// NewGoldskyClient 创建新的 Goldsky 客户端
func NewGoldskyClient(subgraphURL string, chainID int) *GoldskyClient {
	return &GoldskyClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		subgraphURL: subgraphURL,
		chainID:     chainID,
	}
}

// QueryCompoundFlows 查询 Compound Flows
func (c *GoldskyClient) QueryCompoundFlows(ctx context.Context, contractAddresses []string, limit int) ([]types.GoldskyCompoundFlow, error) {
	query := `
		query($contractAddresses: [Bytes!], $limit: Int!) {
			compoundTimelockFlows(
				where: { contractAddress_in: $contractAddresses }
				first: $limit
				orderBy: createdAt
				orderDirection: asc
			) {
				id
				flowId
				timelockStandard
				contractAddress
				status
				queueTransaction {
					id
					txHash
					logIndex
					blockNumber
					blockTimestamp
					contractAddress
					fromAddress
					eventType
					eventTxHash
					eventTarget
					eventValue
					eventSignature
					eventData
					eventEta
				}
				executeTransaction {
					id
					txHash
					blockNumber
					blockTimestamp
					fromAddress
					eventType
				}
				cancelTransaction {
					id
					txHash
					blockNumber
					blockTimestamp
					fromAddress
					eventType
				}
				initiatorAddress
				targetAddress
				value
				callData
				functionSignature
				queuedAt
				eta
				gracePeriod
				expiredAt
				executedAt
				cancelledAt
				createdAt
				updatedAt
			}
		}
	`

	variables := map[string]interface{}{
		"contractAddresses": contractAddresses,
		"limit":             limit,
	}

	var response types.GoldskyCompoundFlowsResponse
	if err := c.executeQuery(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	return response.Data.CompoundTimelockFlows, nil
}

// QueryOpenzeppelinFlows 查询 OpenZeppelin Flows
func (c *GoldskyClient) QueryOpenzeppelinFlows(ctx context.Context, contractAddresses []string, limit int) ([]types.GoldskyOpenzeppelinFlow, error) {
	query := `
		query($contractAddresses: [Bytes!], $limit: Int!) {
			openzeppelinTimelockFlows(
				where: { contractAddress_in: $contractAddresses }
				first: $limit
				orderBy: createdAt
				orderDirection: asc
			) {
				id
				flowId
				timelockStandard
				contractAddress
				status
				scheduleTransaction {
					id
					txHash
					logIndex
					blockNumber
					blockTimestamp
					contractAddress
					fromAddress
					eventType
					eventId
					eventIndex
					eventTarget
					eventValue
					eventData
					eventPredecessor
					eventDelay
				}
				executeTransaction {
					id
					txHash
					blockNumber
					blockTimestamp
					fromAddress
					eventType
				}
				cancelTransaction {
					id
					txHash
					blockNumber
					blockTimestamp
					fromAddress
					eventType
				}
				initiatorAddress
				targetAddress
				value
				callData
				queuedAt
				delay
				eta
				executedAt
				cancelledAt
				createdAt
				updatedAt
			}
		}
	`

	variables := map[string]interface{}{
		"contractAddresses": contractAddresses,
		"limit":             limit,
	}

	var response types.GoldskyOpenzeppelinFlowsResponse
	if err := c.executeQuery(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	return response.Data.OpenzeppelinTimelockFlows, nil
}

// QueryCompoundTransactionByTxHash 根据交易哈希查询 Compound Transaction
func (c *GoldskyClient) QueryCompoundTransactionByTxHash(ctx context.Context, txHash string) (*types.GoldskyCompoundTransaction, error) {
	query := `
		query($txHash: Bytes!) {
			compoundTimelockTransactions(
				where: { txHash: $txHash }
				first: 1
			) {
				id
				txHash
				logIndex
				blockNumber
				blockTimestamp
				contractAddress
				fromAddress
				eventType
				eventTxHash
				eventTarget
				eventValue
				eventSignature
				eventData
				eventEta
			}
		}
	`

	variables := map[string]interface{}{
		"txHash": txHash,
	}

	var response types.GoldskyCompoundTransactionResponse
	if err := c.executeQuery(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	if len(response.Data.CompoundTimelockTransactions) == 0 {
		return nil, nil
	}

	return &response.Data.CompoundTimelockTransactions[0], nil
}

// QueryOpenzeppelinTransactionByTxHash 根据交易哈希查询 OpenZeppelin Transaction
func (c *GoldskyClient) QueryOpenzeppelinTransactionByTxHash(ctx context.Context, txHash string) (*types.GoldskyOpenzeppelinTransaction, error) {
	query := `
		query($txHash: Bytes!) {
			openzeppelinTimelockTransactions(
				where: { txHash: $txHash }
				first: 1
			) {
				id
				txHash
				logIndex
				blockNumber
				blockTimestamp
				contractAddress
				fromAddress
				eventType
				eventId
				eventIndex
				eventTarget
				eventValue
				eventData
				eventPredecessor
				eventDelay
			}
		}
	`

	variables := map[string]interface{}{
		"txHash": txHash,
	}

	var response types.GoldskyOpenzeppelinTransactionResponse
	if err := c.executeQuery(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	if len(response.Data.OpenzeppelinTimelockTransactions) == 0 {
		return nil, nil
	}

	return &response.Data.OpenzeppelinTimelockTransactions[0], nil
}

// executeQuery 执行 GraphQL 查询
func (c *GoldskyClient) executeQuery(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	requestBody := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		logger.Error("Failed to marshal GraphQL query", err)
		return fmt.Errorf("failed to marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.subgraphURL, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Failed to create HTTP request", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to execute GraphQL query", err, "url", c.subgraphURL)
		return fmt.Errorf("failed to execute query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Error("GraphQL query failed", fmt.Errorf("status code: %d", resp.StatusCode), "body", string(body))
		return fmt.Errorf("query failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", err)
		return fmt.Errorf("failed to read response: %w", err)
	}

	if err := json.Unmarshal(body, result); err != nil {
		logger.Error("Failed to unmarshal GraphQL response", err, "body", string(body))
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// ConvertGoldskyCompoundFlowToDB 转换 Goldsky Compound Flow 为数据库模型
func ConvertGoldskyCompoundFlowToDB(goldskyFlow types.GoldskyCompoundFlow, chainID int) (*types.CompoundTimelockFlowDB, error) {
	flow := &types.CompoundTimelockFlowDB{
		FlowID:           goldskyFlow.FlowID,
		TimelockStandard: "compound",
		ChainID:          chainID,
		ContractAddress:  goldskyFlow.ContractAddress,
		Status:           goldskyFlow.Status,
		Value:            goldskyFlow.Value,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// 处理交易哈希
	if goldskyFlow.QueueTransaction != nil {
		txHash := goldskyFlow.QueueTransaction.TxHash
		flow.QueueTxHash = &txHash
		flow.InitiatorAddress = &goldskyFlow.QueueTransaction.FromAddress
	}
	if goldskyFlow.ExecuteTransaction != nil {
		txHash := goldskyFlow.ExecuteTransaction.TxHash
		flow.ExecuteTxHash = &txHash
	}
	if goldskyFlow.CancelTransaction != nil {
		txHash := goldskyFlow.CancelTransaction.TxHash
		flow.CancelTxHash = &txHash
	}

	// 处理地址
	if goldskyFlow.InitiatorAddress != nil {
		flow.InitiatorAddress = goldskyFlow.InitiatorAddress
	}
	if goldskyFlow.TargetAddress != nil {
		flow.TargetAddress = goldskyFlow.TargetAddress
	}

	// 处理 CallData (hex string -> bytes)
	if goldskyFlow.CallData != nil && *goldskyFlow.CallData != "" {
		callDataStr := strings.TrimPrefix(*goldskyFlow.CallData, "0x")
		callDataBytes, err := hex.DecodeString(callDataStr)
		if err != nil {
			logger.Error("Failed to decode callData", err, "callData", *goldskyFlow.CallData)
		} else {
			flow.CallData = callDataBytes
		}
	}

	// 处理函数签名
	if goldskyFlow.FunctionSignature != nil {
		flow.FunctionSignature = goldskyFlow.FunctionSignature
	}

	// 处理时间戳 (Unix timestamp string -> time.Time)
	if goldskyFlow.QueuedAt != nil {
		if t, err := parseTimestamp(*goldskyFlow.QueuedAt); err == nil {
			flow.QueuedAt = &t
		}
	}
	if goldskyFlow.Eta != nil {
		if t, err := parseTimestamp(*goldskyFlow.Eta); err == nil {
			flow.Eta = &t
		}
	}
	if goldskyFlow.GracePeriod != nil {
		if gp, err := strconv.ParseInt(*goldskyFlow.GracePeriod, 10, 64); err == nil {
			flow.GracePeriod = &gp
		}
	}
	if goldskyFlow.ExpiredAt != nil {
		if t, err := parseTimestamp(*goldskyFlow.ExpiredAt); err == nil {
			flow.ExpiredAt = &t
		}
	}
	if goldskyFlow.ExecutedAt != nil {
		if t, err := parseTimestamp(*goldskyFlow.ExecutedAt); err == nil {
			flow.ExecutedAt = &t
		}
	}
	if goldskyFlow.CancelledAt != nil {
		if t, err := parseTimestamp(*goldskyFlow.CancelledAt); err == nil {
			flow.CancelledAt = &t
		}
	}

	return flow, nil
}

// ConvertGoldskyOpenzeppelinFlowToDB 转换 Goldsky OpenZeppelin Flow 为数据库模型
func ConvertGoldskyOpenzeppelinFlowToDB(goldskyFlow types.GoldskyOpenzeppelinFlow, chainID int) (*types.OpenzeppelinTimelockFlowDB, error) {
	flow := &types.OpenzeppelinTimelockFlowDB{
		FlowID:           goldskyFlow.FlowID,
		TimelockStandard: "openzeppelin",
		ChainID:          chainID,
		ContractAddress:  goldskyFlow.ContractAddress,
		Status:           goldskyFlow.Status,
		Value:            goldskyFlow.Value,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// 处理交易哈希
	if goldskyFlow.ScheduleTransaction != nil {
		txHash := goldskyFlow.ScheduleTransaction.TxHash
		flow.ScheduleTxHash = &txHash
		flow.InitiatorAddress = &goldskyFlow.ScheduleTransaction.FromAddress
	}
	if goldskyFlow.ExecuteTransaction != nil {
		txHash := goldskyFlow.ExecuteTransaction.TxHash
		flow.ExecuteTxHash = &txHash
	}
	if goldskyFlow.CancelTransaction != nil {
		txHash := goldskyFlow.CancelTransaction.TxHash
		flow.CancelTxHash = &txHash
	}

	// 处理地址
	if goldskyFlow.InitiatorAddress != nil {
		flow.InitiatorAddress = goldskyFlow.InitiatorAddress
	}
	if goldskyFlow.TargetAddress != nil {
		flow.TargetAddress = goldskyFlow.TargetAddress
	}

	// 处理 CallData (hex string -> bytes)
	if goldskyFlow.CallData != nil && *goldskyFlow.CallData != "" {
		callDataStr := strings.TrimPrefix(*goldskyFlow.CallData, "0x")
		callDataBytes, err := hex.DecodeString(callDataStr)
		if err != nil {
			logger.Error("Failed to decode callData", err, "callData", *goldskyFlow.CallData)
		} else {
			flow.CallData = callDataBytes
		}
	}

	// 处理时间戳
	if goldskyFlow.QueuedAt != nil {
		if t, err := parseTimestamp(*goldskyFlow.QueuedAt); err == nil {
			flow.QueuedAt = &t
		}
	}
	if goldskyFlow.Delay != nil {
		if d, err := strconv.ParseInt(*goldskyFlow.Delay, 10, 64); err == nil {
			flow.Delay = &d
		}
	}
	if goldskyFlow.Eta != nil {
		if t, err := parseTimestamp(*goldskyFlow.Eta); err == nil {
			flow.Eta = &t
		}
	}
	if goldskyFlow.ExecutedAt != nil {
		if t, err := parseTimestamp(*goldskyFlow.ExecutedAt); err == nil {
			flow.ExecutedAt = &t
		}
	}
	if goldskyFlow.CancelledAt != nil {
		if t, err := parseTimestamp(*goldskyFlow.CancelledAt); err == nil {
			flow.CancelledAt = &t
		}
	}

	return flow, nil
}

// QueryGlobalStatistics 查询全局统计数据
func (c *GoldskyClient) QueryGlobalStatistics(ctx context.Context) (*GlobalStatistics, error) {
	query := `
		query {
			globalStatistics(id: "global") {
				id
				totalCompoundContracts
				totalOpenzeppelinContracts
				totalContracts
				totalCompoundFlows
				totalOpenzeppelinFlows
				totalFlows
				activeCompoundFlows
				activeOpenzeppelinFlows
				activeFlows
				totalCompoundTransactions
				totalOpenzeppelinTransactions
				totalTransactions
				lastUpdatedAt
			}
		}
	`

	resp := struct {
		Data struct {
			GlobalStatistics *GlobalStatistics `json:"globalStatistics"`
		} `json:"data"`
	}{}

	if err := c.executeQuery(ctx, query, nil, &resp); err != nil {
		return nil, fmt.Errorf("failed to query global statistics: %w", err)
	}

	if resp.Data.GlobalStatistics == nil {
		return nil, fmt.Errorf("global statistics not found")
	}

	logger.Info("Queried global statistics from Goldsky", "chain_id", c.chainID, "total_contracts", resp.Data.GlobalStatistics.TotalContracts, "total_transactions", resp.Data.GlobalStatistics.TotalTransactions)
	return resp.Data.GlobalStatistics, nil
}

// GlobalStatistics Goldsky返回的全局统计数据
type GlobalStatistics struct {
	ID                            string `json:"id"`
	TotalCompoundContracts        string `json:"totalCompoundContracts"`
	TotalOpenzeppelinContracts    string `json:"totalOpenzeppelinContracts"`
	TotalContracts                string `json:"totalContracts"`
	TotalCompoundFlows            string `json:"totalCompoundFlows"`
	TotalOpenzeppelinFlows        string `json:"totalOpenzeppelinFlows"`
	TotalFlows                    string `json:"totalFlows"`
	ActiveCompoundFlows           string `json:"activeCompoundFlows"`
	ActiveOpenzeppelinFlows       string `json:"activeOpenzeppelinFlows"`
	ActiveFlows                   string `json:"activeFlows"`
	TotalCompoundTransactions     string `json:"totalCompoundTransactions"`
	TotalOpenzeppelinTransactions string `json:"totalOpenzeppelinTransactions"`
	TotalTransactions             string `json:"totalTransactions"`
	LastUpdatedAt                 string `json:"lastUpdatedAt"`
}

// parseTimestamp 解析时间戳字符串为 time.Time
func parseTimestamp(ts string) (time.Time, error) {
	timestamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(timestamp, 0), nil
}
