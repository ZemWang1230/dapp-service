# 扫链系统重写总结

## 重写目标
基于新修改的数据库表结构和类型定义，重写扫链系统，实现以下流程：
1. 使用 `eth_getLogs` (FilterLogs) 获取事件数据
2. 使用 `eth_getTransactionByHash` 和 `tx.Sender()` 获取交易发送者地址
3. 使用 `eth_getTransactionReceipt` 获取交易状态
4. 使用 `eth_getBlockByNumber` 获取区块时间戳（适配不同链）
5. 实现 ABI 解码解析事件参数
6. 整合信息并写入数据库

## 主要改进

### 1. 区块处理器 (BlockProcessor)
- **新增功能**: 
  - 使用 `eth_getLogs` 批量获取事件，替代逐个区块扫描
  - 内置 Compound 和 OpenZeppelin Timelock 的 ABI 定义
  - 完整的事件解析和字段提取
  - 支持不同链的区块获取（为 ZKSync 等特殊链预留扩展空间）

- **核心方法**:
  - `ScanBlockRange()`: 扫描指定区块范围的所有相关事件
  - `processLog()`: 处理单个日志事件，包含完整的数据获取流程
  - `getTransactionSender()`: 使用签名恢复获取交易发送者
  - `getBlockByNumberSafe()`: 安全获取区块信息，适配不同链

### 2. 事件处理优化
- **完整数据映射**: 将解析的事件数据完整映射到数据库结构
- **ABI 解码**: 正确解析 indexed 和 non-indexed 参数
- **错误处理**: 改进错误处理和日志记录

### 3. 链扫描器 (ChainScanner)
- **批量扫描**: 使用区块范围扫描替代单区块扫描，提高效率
- **简化流程**: 移除不必要的中间步骤

## 新的扫链流程

### 流程图
```
开始扫描
    ↓
获取区块范围 (fromBlock, toBlock)
    ↓
eth_getLogs 获取所有相关事件
    ↓
对每个事件并行处理：
    ├── eth_getTransactionByHash → 获取交易信息
    ├── eth_getTransactionReceipt → 获取交易状态
    ├── eth_getBlockByNumber → 获取区块时间戳
    └── ABI解码 → 解析事件参数
    ↓
整合数据并批量写入数据库
    ↓
更新扫描进度
```

### 详细步骤

1. **事件发现**
   ```go
   // 使用 FilterQuery 获取所有 Timelock 相关事件
   query := ethereum.FilterQuery{
       FromBlock: big.NewInt(fromBlock),
       ToBlock:   big.NewInt(toBlock),
       Topics:    [][]common.Hash{topics}, // 包含所有事件签名
   }
   logs, err := client.FilterLogs(ctx, query)
   ```

2. **交易信息获取**
   ```go
   // 获取交易详情
   tx, _, err := client.TransactionByHash(ctx, log.TxHash)
   
   // 获取发送者地址
   signer := ethtypes.LatestSignerForChainID(chainID)
   sender, err := ethtypes.Sender(signer, tx)
   
   // 获取交易回执
   receipt, err := client.TransactionReceipt(ctx, log.TxHash)
   ```

3. **区块信息获取**
   ```go
   // 安全获取区块信息（适配不同链）
   block, err := client.BlockByNumber(ctx, big.NewInt(blockNumber))
   ```

4. **ABI 解码**
   ```go
   // 解析事件数据
   eventData := make(map[string]interface{})
   err := abiEvent.Inputs.UnpackIntoMap(eventData, log.Data)
   
   // 解析 indexed 参数
   for _, input := range abiEvent.Inputs {
       if input.Indexed {
           // 处理 indexed 参数
       }
   }
   ```

## 支持的事件类型

### Compound Timelock 事件
- `QueueTransaction(bytes32 indexed txHash, address indexed target, uint256 value, string signature, bytes data, uint256 eta)`
- `ExecuteTransaction(bytes32 indexed txHash, address indexed target, uint256 value, string signature, bytes data, uint256 eta)`
- `CancelTransaction(bytes32 indexed txHash, address indexed target, uint256 value, string signature, bytes data, uint256 eta)`

### OpenZeppelin TimelockController 事件
- `CallScheduled(bytes32 indexed id, uint256 indexed index, address target, uint256 value, bytes data, bytes32 predecessor, uint256 delay)`
- `CallExecuted(bytes32 indexed id, uint256 indexed index, address target, uint256 value, bytes data)`
- `Cancelled(bytes32 indexed id)`

## 数据库存储

### Compound Timelock 交易表
- 完整的事件数据存储，包括解析后的参数
- 支持 JSON 格式的事件数据存储
- 包含交易状态和区块时间戳

### OpenZeppelin Timelock 交易表
- 支持复杂的事件结构
- 正确处理 indexed 和 non-indexed 参数
- 完整的流程追踪支持

## 性能优化

1. **批量扫描**: 使用 `eth_getLogs` 一次获取整个区块范围的事件
2. **并行处理**: 事件处理可以并行进行
3. **批量写入**: 数据库操作使用批量插入
4. **错误恢复**: 改进的错误处理和重试机制

## 链适配性

系统设计支持不同的 EVM 兼容链：
- **通用方法**: 使用标准的 JSON-RPC 方法
- **特殊链支持**: 为 ZKSync 等特殊链预留扩展接口
- **错误处理**: 针对不同链的特殊错误进行处理

## 配置说明

扫描器配置保持不变，但新增了以下特性：
- 更高效的批量扫描
- 改进的错误处理
- 更好的监控和状态报告

## 迁移说明

从旧系统迁移到新系统：
1. 数据库结构已更新，支持更丰富的事件数据
2. 扫描逻辑完全重写，性能更优
3. 保持现有的 API 接口兼容性
4. 支持从任意区块重新开始扫描

## 总结

重写后的扫链系统具有以下优势：
- **高效**: 使用批量事件获取，减少 RPC 调用次数
- **准确**: 完整的 ABI 解码，确保数据准确性
- **可靠**: 改进的错误处理和恢复机制
- **可扩展**: 支持新的链和事件类型
- **可维护**: 清晰的代码结构和完整的日志记录