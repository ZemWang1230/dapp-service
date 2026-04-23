package scanner

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Multicall3Address 是 Multicall3 在绝大多数 EVM 网络（主网、测试网、L2）上
// 的预部署地址。参考：https://github.com/mds1/multicall
var Multicall3Address = common.HexToAddress("0xcA11bde05977b3631167028862bE2a173976CA11")

// multicall3ABI 只包含我们用到的 aggregate3 方法
const multicall3ABI = `[
	{
		"inputs": [
			{
				"components": [
					{"internalType":"address","name":"target","type":"address"},
					{"internalType":"bool","name":"allowFailure","type":"bool"},
					{"internalType":"bytes","name":"callData","type":"bytes"}
				],
				"internalType": "struct Multicall3.Call3[]",
				"name": "calls",
				"type": "tuple[]"
			}
		],
		"name": "aggregate3",
		"outputs": [
			{
				"components": [
					{"internalType":"bool","name":"success","type":"bool"},
					{"internalType":"bytes","name":"returnData","type":"bytes"}
				],
				"internalType": "struct Multicall3.Result[]",
				"name": "returnData",
				"type": "tuple[]"
			}
		],
		"stateMutability": "payable",
		"type": "function"
	}
]`

var (
	parsedMulticallABI abi.ABI
	// 懒加载初始化错误，避免包初始化阶段 panic
	multicallInitErr error
)

func init() {
	parsedMulticallABI, multicallInitErr = abi.JSON(strings.NewReader(multicall3ABI))
}

// Call3 对应 Multicall3.Call3 的一次子调用
type Call3 struct {
	Target       common.Address
	AllowFailure bool
	CallData     []byte
}

// Call3Result 对应 Multicall3.Result 返回
type Call3Result struct {
	Success    bool
	ReturnData []byte
}

// call3ABIItem 是 ABI 打包时需要的匿名结构（字段顺序与 tuple 一致）
type call3ABIItem struct {
	Target       common.Address
	AllowFailure bool
	CallData     []byte
}

// call3ResultABIItem 对应解包结果
type call3ResultABIItem struct {
	Success    bool
	ReturnData []byte
}

// AggregateCall3 用 Multicall3.aggregate3 一次性批量调用多个合约方法。
//
// 用法：先把每个子调用用对应合约的 ABI 打包成 callData，再交给本函数汇总一次发往链上。
// 返回结果与输入按索引对齐。
func AggregateCall3(ctx context.Context, client *ethclient.Client, calls []Call3) ([]Call3Result, error) {
	if multicallInitErr != nil {
		return nil, fmt.Errorf("failed to parse multicall3 abi: %w", multicallInitErr)
	}
	if len(calls) == 0 {
		return nil, nil
	}

	items := make([]call3ABIItem, len(calls))
	for i, c := range calls {
		items[i] = call3ABIItem{
			Target:       c.Target,
			AllowFailure: c.AllowFailure,
			CallData:     c.CallData,
		}
	}

	packed, err := parsedMulticallABI.Pack("aggregate3", items)
	if err != nil {
		return nil, fmt.Errorf("failed to pack multicall3 aggregate3: %w", err)
	}

	to := Multicall3Address
	raw, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &to,
		Data: packed,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call multicall3: %w", err)
	}

	decoded, err := parsedMulticallABI.Unpack("aggregate3", raw)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack multicall3 aggregate3: %w", err)
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("empty multicall3 result")
	}

	// decoded[0] 类型是 []struct{ Success bool; ReturnData []byte }（匿名结构）
	// 走 abi.ConvertType 得到强类型切片
	raws, ok := decoded[0].([]struct {
		Success    bool   `json:"success"`
		ReturnData []byte `json:"returnData"`
	})
	if !ok {
		// 旧版本/不同版本 go-ethereum 可能返回不同的匿名结构名，退回反射式转换
		converted := abi.ConvertType(decoded[0], new([]call3ResultABIItem))
		list, okConv := converted.(*[]call3ResultABIItem)
		if !okConv {
			return nil, fmt.Errorf("unexpected multicall3 return type: %T", decoded[0])
		}
		out := make([]Call3Result, len(*list))
		for i, r := range *list {
			out[i] = Call3Result{Success: r.Success, ReturnData: r.ReturnData}
		}
		return out, nil
	}

	out := make([]Call3Result, len(raws))
	for i, r := range raws {
		out[i] = Call3Result{Success: r.Success, ReturnData: r.ReturnData}
	}
	return out, nil
}
