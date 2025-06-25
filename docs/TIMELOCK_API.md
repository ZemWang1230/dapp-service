# TimeLocker API 文档

## 概述

本文档描述了TimeLocker项目中与timelock合约相关的API端点。所有API都基于钱包地址进行操作，支持Compound和OpenZeppelin两种timelock合约标准。

## 认证

所有timelock相关的API都需要JWT认证。请在请求头中包含：

```
Authorization: Bearer <access_token>
```

## API端点

### 1. 检查TimeLocker状态

检查用户是否有timelock合约，用于决定显示创建/导入页面还是timelock列表页面。

**端点:** `GET /api/v1/timelock/status`

**响应:**
```json
{
  "success": true,
  "data": {
    "has_timelocks": true,
    "timelocks": [
      {
        "id": 1,
        "wallet_address": "0x1234...",
        "chain_id": 1,
        "chain_name": "eth-mainnet",
        "contract_address": "0x5678...",
        "standard": "compound",
        "creator_address": "0x1234...",
        "tx_hash": "0xabcd...",
        "min_delay": 172800,
        "admin": "0x9999...",
        "remark": "主网timelock合约",
        "status": "active",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
      }
    ]
  }
}
```

### 2. 创建TimeLocker合约

前端创建合约后，将合约信息存储到数据库。

**端点:** `POST /api/v1/timelock/create`

**请求体 (Compound标准):**
```json
{
  "chain_id": 1,
  "chain_name": "eth-mainnet",
  "contract_address": "0x1234567890123456789012345678901234567890",
  "standard": "compound",
  "creator_address": "0x1234567890123456789012345678901234567890",
  "tx_hash": "0x1234567890123456789012345678901234567890123456789012345678901234",
  "min_delay": 172800,
  "admin": "0x1234567890123456789012345678901234567890",
  "remark": "主网timelock合约"
}
```

**请求体 (OpenZeppelin标准):**
```json
{
  "chain_id": 1,
  "chain_name": "eth-mainnet",
  "contract_address": "0x1234567890123456789012345678901234567890",
  "standard": "openzeppelin",
  "creator_address": "0x1234567890123456789012345678901234567890",
  "tx_hash": "0x1234567890123456789012345678901234567890123456789012345678901234",
  "min_delay": 172800,
  "proposers": [
    "0x1111111111111111111111111111111111111111",
    "0x2222222222222222222222222222222222222222"
  ],
  "executors": [
    "0x3333333333333333333333333333333333333333",
    "0x4444444444444444444444444444444444444444"
  ],
  "remark": "主网timelock合约"
}
```

### 3. 导入TimeLocker合约

导入已存在的timelock合约，需要验证合约是否为有效的timelock合约。

**端点:** `POST /api/v1/timelock/import`

**请求体:**
```json
{
  "chain_id": 1,
  "chain_name": "eth-mainnet",
  "contract_address": "0x1234567890123456789012345678901234567890",
  "standard": "compound",
  "abi": "[{\"inputs\":[],\"name\":\"delay\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
  "remark": "导入的timelock合约"
}
```

### 4. 获取TimeLocker列表

获取用户的timelock合约列表，支持分页和筛选。

**端点:** `GET /api/v1/timelock/list`

**查询参数:**
- `page`: 页码 (默认: 1)
- `page_size`: 每页数量 (默认: 10, 最大: 100)
- `chain_id`: 链ID筛选 (可选)
- `standard`: 合约标准筛选 (可选: compound, openzeppelin)
- `status`: 状态筛选 (可选: active, inactive)

**示例:** `GET /api/v1/timelock/list?page=1&page_size=10&chain_id=1&standard=compound`

**响应:**
```json
{
  "success": true,
  "data": {
    "list": [
      {
        "id": 1,
        "wallet_address": "0x1234...",
        "chain_id": 1,
        "chain_name": "eth-mainnet",
        "contract_address": "0x5678...",
        "standard": "compound",
        "min_delay": 172800,
        "remark": "主网timelock合约",
        "status": "active",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

### 5. 获取TimeLocker详情

获取指定timelock合约的详细信息。

**端点:** `GET /api/v1/timelock/{id}`

**响应:**
```json
{
  "success": true,
  "data": {
    "id": 1,
    "wallet_address": "0x1234...",
    "chain_id": 1,
    "chain_name": "eth-mainnet",
    "contract_address": "0x5678...",
    "standard": "openzeppelin",
    "min_delay": 172800,
    "proposers": "[\"0x1111...\", \"0x2222...\"]",
    "executors": "[\"0x3333...\", \"0x4444...\"]",
    "remark": "主网timelock合约",
    "status": "active",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
    "proposers_list": ["0x1111...", "0x2222..."],
    "executors_list": ["0x3333...", "0x4444..."]
  }
}
```

### 6. 更新TimeLocker

更新timelock合约的备注信息。

**端点:** `PUT /api/v1/timelock/{id}`

**请求体:**
```json
{
  "remark": "更新后的备注信息"
}
```

**响应:**
```json
{
  "success": true,
  "data": {
    "message": "Timelock updated successfully"
  }
}
```

### 7. 删除TimeLocker

删除timelock合约（软删除）。

**端点:** `DELETE /api/v1/timelock/{id}`

**响应:**
```json
{
  "success": true,
  "data": {
    "message": "Timelock deleted successfully"
  }
}
```

## 错误响应

所有API在发生错误时都会返回统一的错误格式：

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "错误描述",
    "details": "详细错误信息（可选）"
  }
}
```

### 常见错误码

- `UNAUTHORIZED`: 用户未认证
- `INVALID_REQUEST`: 请求参数无效
- `TIMELOCK_EXISTS`: timelock合约已存在
- `TIMELOCK_NOT_FOUND`: timelock合约不存在
- `UNAUTHORIZED_ACCESS`: 无权访问该timelock合约
- `INVALID_CONTRACT`: 无效的timelock合约
- `INVALID_STANDARD`: 无效的合约标准
- `INVALID_PARAMETERS`: 无效的合约参数
- `INVALID_REMARK`: 无效的备注内容
- `INTERNAL_ERROR`: 服务器内部错误

## 数据库表结构

TimeLocker使用以下数据库表：

### timelocks表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGSERIAL | 主键 |
| wallet_address | VARCHAR(42) | 钱包地址 |
| chain_id | INTEGER | 链ID |
| chain_name | VARCHAR(50) | 链名称 |
| contract_address | VARCHAR(42) | 合约地址 |
| standard | VARCHAR(20) | 合约标准 (compound/openzeppelin) |
| creator_address | VARCHAR(42) | 创建者地址 |
| tx_hash | VARCHAR(66) | 创建交易hash |
| min_delay | BIGINT | 最小延迟时间（秒） |
| proposers | TEXT | 提议者地址列表（JSON） |
| executors | TEXT | 执行者地址列表（JSON） |
| admin | VARCHAR(42) | 管理员地址 |
| remark | VARCHAR(500) | 备注 |
| status | VARCHAR(20) | 状态 (active/inactive/deleted) |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

## 安全性

1. **输入验证**: 所有输入都经过严格验证，包括地址格式、参数范围等
2. **XSS防护**: 备注内容经过HTML转义处理
3. **SQL注入防护**: 使用参数化查询和正则表达式检查
4. **权限控制**: 用户只能操作自己的timelock合约
5. **合约验证**: 导入时验证ABI中包含必要的timelock函数

## 使用流程

1. **新用户流程**:
   - 调用 `/status` 检查状态
   - 如果 `has_timelocks` 为 false，显示创建/导入选择页面
   - 用户选择创建或导入timelock合约

2. **已有timelock用户流程**:
   - 调用 `/status` 检查状态
   - 如果 `has_timelocks` 为 true，直接显示timelock列表
   - 用户可以查看、更新、删除已有的timelock合约

3. **操作确认**:
   - 所有操作都在用户点击确认后才会存储到数据库
   - 创建和导入操作会返回完整的timelock信息 