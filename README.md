# TimeLocker Backend v2.1

基于Covalent API的区块链资产管理平台后端服务 - 优化版。

## 主要特性

- 🔐 以钱包地址为核心的用户认证
- 🔗 多链支持，用户可切换链进行timelock合约操作
- 🌐 支持多链资产查询（基于Covalent API，包括测试网）
- 💰 实时获取代币余额和价格信息
- 📈 24小时价格涨跌幅显示
- 🎯 智能排序：主网资产按价值排序，测试网仅显示原生代币且不计入总价值
- 🖼️ 包含链Logo和代币Logo信息
- 📊 资产组合管理和价值统计
- 🚀 高性能缓存机制
- 📝 完整的API文档

## 支持的区块链

### 主网
- Ethereum Mainnet (eth-mainnet)
- Polygon Mainnet (matic-mainnet)
- Avalanche C-Chain (avalanche-mainnet)
- BNB Smart Chain (bsc-mainnet)
- Arbitrum One (arbitrum-mainnet)
- Optimism (optimism-mainnet)
- Base (base-mainnet)
- Fantom (fantom-mainnet)
- Moonbeam (moonbeam-mainnet)

### 测试网
- Ethereum Sepolia (eth-sepolia)
- Polygon Mumbai (matic-mumbai)
- Avalanche Fuji (avalanche-testnet)
- BNB Smart Chain Testnet (bsc-testnet)
- Arbitrum Sepolia (arbitrum-sepolia)
- Optimism Sepolia (optimism-sepolia)
- Base Sepolia (base-sepolia)

## 技术栈

- **语言**: Go 1.21+
- **框架**: Gin
- **数据库**: PostgreSQL
- **缓存**: Redis
- **API**: Covalent API
- **文档**: Swagger
- **认证**: JWT

## 快速开始

### 1. 环境要求

- Go 1.21+
- PostgreSQL 12+
- Redis 6+
- Covalent API Key

### 2. 获取Covalent API Key

1. 访问 [Covalent官网](https://www.covalenthq.com/)
2. 注册账户并获取免费的API Key
3. 将API Key配置到 `config.yaml` 文件中

### 3. 安装依赖

```bash
go mod download
```

### 4. 配置数据库

```bash
# 创建数据库
createdb timelocker_db

# 执行初始化脚本
psql -d timelocker_db -f pkg/database/init.sql
```

### 5. 配置文件

修改 `config.yaml` 中的Covalent API Key：

```yaml
covalent:
  api_key: "your-covalent-api-key-here"
```

### 6. 启动服务

```bash
go run cmd/server/main.go
```

服务将在 `http://localhost:8080` 启动。

## API文档

启动服务后，访问 `http://localhost:8080/swagger/index.html` 查看完整的API文档。

### 主要API端点

#### 认证相关
- `POST /api/v1/auth/wallet-connect` - 钱包连接登录（支持链ID）
- `POST /api/v1/auth/switch-chain` - 切换链（需要重新签名）
- `POST /api/v1/auth/refresh` - 刷新Token
- `GET /api/v1/auth/profile` - 获取用户资料

#### 资产相关
- `GET /api/v1/assets` - 获取用户资产（自动刷新）
- `POST /api/v1/assets/refresh` - 手动刷新用户资产

### 使用示例

#### 1. 钱包连接登录

```bash
curl -X POST http://localhost:8080/api/v1/auth/wallet-connect \
  -H "Content-Type: application/json" \
  -d '{
    "wallet_address": "0x742C3cF9Af45f91B109A81EfEaf11535ECDe24C5",
    "signature": "0x...",
    "message": "Connect to TimeLocker",
    "chain_id": 1
  }'
```

#### 2. 切换链（timelock合约操作需要）

```bash
curl -X POST http://localhost:8080/api/v1/auth/switch-chain \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-jwt-token" \
  -d '{
    "chain_id": 137,
    "signature": "0x...",
    "message": "Switch to Polygon chain for timelock operations"
  }'
```

#### 3. 获取资产信息（自动刷新）

```bash
curl -X GET "http://localhost:8080/api/v1/assets" \
  -H "Authorization: Bearer your-jwt-token"
```

#### 4. 手动刷新资产信息

```bash
curl -X POST http://localhost:8080/api/v1/assets/refresh \
  -H "Authorization: Bearer your-jwt-token"
```

### 响应示例（包含24h涨幅和智能排序）

```json
{
  "success": true,
  "data": {
    "wallet_address": "0x742C3cF9Af45f91B109A81EfEaf11535ECDe24C5",
    "total_usd_value": 2850.298,
    "assets": [
      {
        "chain_name": "eth-mainnet",
        "chain_display_name": "Ethereum Mainnet",
        "chain_id": 1,
        "contract_address": "",
        "token_symbol": "ETH",
        "token_name": "Ethereum",
        "token_decimals": 18,
        "balance": "1.23456789",
        "balance_wei": "1234567890000000000",
        "usd_value": 2500.123,
        "token_price": 2025.45,
        "price_change_24h": 5.25,
        "is_native": true,
        "is_testnet": false,
        "token_logo_url": "https://logos.covalenthq.com/tokens/1/0x0.png",
        "chain_logo_url": "https://logos.covalenthq.com/chains/1.png",
        "last_updated": "2024-01-01T12:00:00Z"
      },
             {
         "chain_name": "matic-mainnet",
         "chain_display_name": "Polygon Mainnet",
         "chain_id": 137,
         "contract_address": "",
         "token_symbol": "MATIC",
         "token_name": "Polygon",
         "token_decimals": 18,
         "balance": "500.25",
         "balance_wei": "500250000000000000000",
         "usd_value": 350.175,
         "token_price": 0.70,
         "price_change_24h": 2.15,
         "is_native": true,
         "is_testnet": false,
         "token_logo_url": "https://logos.covalenthq.com/tokens/137/0x0000000000000000000000000000000000001010.png",
         "chain_logo_url": "https://logos.covalenthq.com/chains/137.png",
         "last_updated": "2024-01-01T12:00:00Z"
       },
      {
                 "chain_name": "eth-sepolia",
         "chain_display_name": "Ethereum Sepolia",
         "chain_id": 11155111,
         "contract_address": "",
         "token_symbol": "ETH",
         "token_name": "Ethereum",
         "token_decimals": 18,
         "balance": "5.0",
         "balance_wei": "5000000000000000000",
         "usd_value": 0,
         "token_price": 0,
         "price_change_24h": 0,
         "is_native": true,
         "is_testnet": true,
         "token_logo_url": "https://logos.covalenthq.com/tokens/11155111/0x0.png",
         "chain_logo_url": "https://logos.covalenthq.com/chains/11155111.png",
         "last_updated": "2024-01-01T12:00:00Z"
       }
    ],
    "last_updated": "2024-01-01T12:00:00Z"
  }
}
```

**新功能说明：**
- ✅ **24h涨幅**: `price_change_24h` 字段显示24小时价格变化百分比
- ✅ **智能排序**: 主网资产按USD价值从高到低排序，测试网资产显示在后面
- ✅ **测试网支持**: 测试网仅显示原生代币（ETH、MATIC、BNB等），USD价值为0，不计入总价值
- ✅ **完整Logo**: 包含代币和链的Logo URL

## 项目结构

```
timelocker-backend/
├── cmd/server/              # 主入口
├── internal/
│   ├── api/                 # API处理器
│   ├── config/              # 配置管理
│   ├── middleware/          # 中间件
│   ├── repository/          # 数据访问层
│   ├── service/             # 业务逻辑层
│   └── types/               # 类型定义
├── pkg/
│   ├── database/            # 数据库工具
│   ├── logger/              # 日志工具
│   └── utils/               # 通用工具
├── docs/                    # 文档
└── config.yaml              # 配置文件
```

## 数据库设计

### 核心表结构

1. **users** - 用户表（简化版）
   - 以 `wallet_address` 为主键
   - 移除了复杂的偏好设置

2. **support_chains** - 支持的区块链
   - 包含Covalent的 `chain_name`
   - 支持主网和测试网标识
   - 包含链Logo信息

3. **user_assets** - 用户资产
   - 从Covalent API获取并缓存
   - 包含代币和链的Logo信息
   - 按USD价值排序

## 核心优化

### ✅ 已完成的优化

1. **API URL格式** - 使用 `https://api.covalenthq.com/v1/{chainName}/address/{walletAddress}/balances_v2/`
2. **Logo支持** - 包含链Logo和代币Logo
3. **测试网支持** - 支持多个测试网络
4. **简化API** - 移除 `force_refresh` 参数，自动刷新逻辑
5. **优化数据库** - 简化表结构，提升性能
6. **chainName标识** - 使用Covalent标准的链名称

### 🚀 架构亮点

- **智能缓存** - 首次访问自动刷新，提升用户体验
- **多网络支持** - 一键支持主网和测试网
- **Logo集成** - 完整的视觉资源支持
- **简化接口** - 更少的参数，更好的体验
- **高效存储** - 优化的数据库结构

## 环境变量

可以通过环境变量覆盖配置：

```bash
export TIMELOCKER_COVALENT_API_KEY="your-api-key"
export TIMELOCKER_DATABASE_HOST="localhost"
export TIMELOCKER_REDIS_HOST="localhost"
```

## 开发指南

### 添加新的区块链支持

1. 在 `pkg/database/init.sql` 中添加新的链信息：
```sql
INSERT INTO support_chains (chain_name, display_name, chain_id, native_token, is_testnet, is_active) 
VALUES ('new-chain', 'New Chain', 12345, 'NEW', false, true);
```

2. 确保Covalent API支持该链
3. 重启服务即可自动支持

### 自定义缓存策略

```yaml
covalent:
  cache_expiry: 300  # 缓存过期时间（秒）
```

## 部署

### Docker部署

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o timelocker-backend cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/timelocker-backend .
COPY --from=builder /app/config.yaml .
CMD ["./timelocker-backend"]
```

## 监控和日志

系统内置结构化日志，可以集成到ELK、Prometheus等监控系统。

## 故障排除

### 常见问题

1. **Covalent API限流**: 
   - 检查API Key配额
   - 增加请求间隔

2. **数据库连接失败**:
   - 检查数据库配置
   - 确保数据库服务运行

3. **Redis连接失败**:
   - 检查Redis服务状态
   - 验证连接配置

4. **链不支持**:
   - 确认Covalent API支持该链
   - 检查数据库中的链配置

## 贡献指南

1. Fork项目
2. 创建功能分支
3. 提交代码
4. 创建Pull Request

## 许可证

MIT License

## 更新日志

### v2.2.0 (最新)
- 📈 **24h涨幅支持**: 添加`price_change_24h`字段，显示代币24小时价格变化百分比
- 🔗 **链ID管理**: 重新添加用户链ID功能，支持timelock合约的链切换
- 🎯 **智能排序**: 资产按USD价值从高到低排序，主网优先，测试网在后
- 🧪 **测试网优化**: 测试网仅显示原生代币，USD价值为0，不计入总价值
- 🔐 **切换链功能**: 新增`/auth/switch-chain`端点，需要重新签名验证
- 📊 **数据完整性**: 确保所有支持的链都能正确显示资产信息

### v2.1.0
- 🔄 使用新的API URL格式 `{chainName}/address/{walletAddress}/balances_v2/`
- 🖼️ 添加链Logo和代币Logo支持
- 🧪 支持测试网络
- 🗃️ 重新设计support_chains表结构
- ⚡ 移除force_refresh参数，优化用户体验
- 🧹 清理不必要的代码，提升性能

### v2.0.0
- 完全重构，基于Covalent API
- 简化架构，提升性能
- 支持更多区块链
- 改进用户体验
