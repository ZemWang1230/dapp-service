# TimeLocker Backend

基于钱包地址的时间锁定后端服务，支持自动价格查询和缓存。

## 功能特性

- 🔐 钱包地址认证和JWT令牌管理
- 💰 多链代币价格实时查询和缓存
- 🚀 高性能Redis缓存
- 📊 自动价格更新服务
- 🔄 支持多价格源（当前支持CoinGecko）
- 📈 支持价格变化趋势

## 技术栈

- **语言**: Go 1.23+
- **框架**: Gin
- **数据库**: PostgreSQL
- **缓存**: Redis
- **价格源**: CoinGecko API
- **认证**: JWT
- **文档**: Swagger

## 快速开始

### 1. 环境准备

确保系统已安装：
- Go 1.23+
- PostgreSQL 12+
- Redis 6+

### 2. 数据库建立

#### 2.1 创建数据库和用户

```bash
# 切换到 postgres 用户（Linux/macOS）
sudo -u postgres psql

# 或者直接连接（如果有权限）
psql -U postgres

# 创建数据库
CREATE DATABASE timelocker_db;

# 创建用户并设置密码
CREATE USER timelocker WITH PASSWORD 'timelocker';

# 授予数据库权限
GRANT ALL PRIVILEGES ON DATABASE timelocker_db TO timelocker;

# 授予创建表和索引的权限
GRANT CREATE ON SCHEMA public TO timelocker;

# 退出 PostgreSQL
\q
```

#### 2.2 数据库表结构

系统包含以下5个核心数据表：

##### 用户表 (users)
```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    wallet_address VARCHAR(42) NOT NULL UNIQUE,
    chain_id INTEGER NOT NULL,
    nonce VARCHAR(255) NOT NULL,
    signature VARCHAR(132),
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

##### 支持的区块链表 (support_chains)
```sql
CREATE TABLE support_chains (
    id BIGSERIAL PRIMARY KEY,
    chain_id INTEGER NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    rpc_provider VARCHAR(50) DEFAULT 'alchemy',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

##### 支持的代币表 (support_tokens)
```sql
CREATE TABLE support_tokens (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    coingecko_id VARCHAR(100) NOT NULL UNIQUE,
    decimals INTEGER NOT NULL DEFAULT 18,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

##### 链代币关联表 (chain_tokens)
```sql
CREATE TABLE chain_tokens (
    id BIGSERIAL PRIMARY KEY,
    chain_id INTEGER NOT NULL,
    token_id INTEGER NOT NULL,
    contract_address VARCHAR(42) DEFAULT '',
    is_native BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(chain_id, token_id),
    FOREIGN KEY (chain_id) REFERENCES support_chains(id),
    FOREIGN KEY (token_id) REFERENCES support_tokens(id)
);
```

##### 用户资产表 (user_assets)
```sql
CREATE TABLE user_assets (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    wallet_address VARCHAR(42) NOT NULL,
    chain_id INTEGER NOT NULL,
    token_id INTEGER NOT NULL,
    balance VARCHAR(78) NOT NULL DEFAULT '0',
    usd_value DECIMAL(20,8) DEFAULT 0,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, chain_id, token_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (chain_id) REFERENCES support_chains(id),
    FOREIGN KEY (token_id) REFERENCES support_tokens(id)
);
```

#### 2.3 自动建表和初始化

系统启动时会自动创建表结构，但您也可以手动执行：

```bash
# 方法1：启动服务自动建表（推荐）
go run cmd/server/main.go

# 方法2：手动连接数据库测试
psql -U timelocker -d timelocker_db -c "\dt"
```

#### 2.4 初始化数据

运行初始化脚本，添加支持的区块链和代币配置：

```bash
# 执行初始化脚本
psql -U timelocker -d timelocker_db -f scripts/init_chains_and_tokens.sql
```

初始化脚本将添加：

**支持的区块链**：
- Ethereum (Chain ID: 1)
- BSC (Chain ID: 56) 
- Polygon (Chain ID: 137)
- Arbitrum One (Chain ID: 42161)

**支持的代币**：
- ETH, BNB, MATIC (原生代币)
- USDC, USDT, DAI, UNI, WETH (ERC-20代币)

**代币合约地址配置**：
每个代币在不同链上的合约地址都已预配置完成。

#### 2.5 验证数据库建立

```bash
# 连接数据库
psql -U timelocker -d timelocker_db

# 查看所有表
\dt

# 查看支持的链
SELECT * FROM support_chains;

# 查看支持的代币  
SELECT * FROM support_tokens;

# 查看链代币配置
SELECT 
    sc.name AS chain_name,
    st.symbol AS token_symbol,
    ct.contract_address,
    ct.is_native
FROM chain_tokens ct
JOIN support_chains sc ON ct.chain_id = sc.id  
JOIN support_tokens st ON ct.token_id = st.id
ORDER BY sc.chain_id, st.symbol;

# 退出
\q
```

#### 2.6 数据库性能优化

系统会自动创建以下索引以提高查询性能：

```sql
-- 用户表索引
CREATE INDEX idx_users_wallet_address ON users(wallet_address);
CREATE INDEX idx_users_chain_id ON users(chain_id);
CREATE INDEX idx_users_created_at ON users(created_at);

-- 代币表索引
CREATE INDEX idx_support_tokens_symbol ON support_tokens(symbol);
CREATE INDEX idx_support_tokens_coingecko_id ON support_tokens(coingecko_id); 
CREATE INDEX idx_support_tokens_is_active ON support_tokens(is_active);

-- 链表索引
CREATE INDEX idx_support_chains_chain_id ON support_chains(chain_id);
CREATE INDEX idx_support_chains_is_active ON support_chains(is_active);

-- 链代币关联表索引
CREATE INDEX idx_chain_tokens_chain_id ON chain_tokens(chain_id);
CREATE INDEX idx_chain_tokens_token_id ON chain_tokens(token_id);
CREATE INDEX idx_chain_tokens_contract_address ON chain_tokens(contract_address);

-- 用户资产表索引
CREATE INDEX idx_user_assets_user_id ON user_assets(user_id);
CREATE INDEX idx_user_assets_wallet_address ON user_assets(wallet_address);
CREATE INDEX idx_user_assets_chain_id ON user_assets(chain_id);
```

### 3. 数据库备份和恢复

#### 3.1 备份数据库

```bash
# 备份整个数据库
pg_dump -U timelocker -h localhost timelocker_db > backup_$(date +%Y%m%d_%H%M%S).sql

# 只备份数据（不包含表结构）
pg_dump -U timelocker -h localhost --data-only timelocker_db > data_backup_$(date +%Y%m%d_%H%M%S).sql

# 只备份表结构（不包含数据）
pg_dump -U timelocker -h localhost --schema-only timelocker_db > schema_backup_$(date +%Y%m%d_%H%M%S).sql
```

#### 3.2 恢复数据库

```bash
# 恢复完整数据库
psql -U timelocker -d timelocker_db < backup_20240101_120000.sql

# 只恢复数据
psql -U timelocker -d timelocker_db < data_backup_20240101_120000.sql
```

### 4. 配置文件

复制并修改配置文件：

```yaml
# config.yaml
server:
  port: "8080"
  mode: "debug"

database:
  host: "localhost"
  port: 5432
  user: "timelocker"
  password: "timelocker"
  dbname: "timelocker_db"
  sslmode: "disable"

redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0

# 区块链RPC配置
rpc:
  provider: "alchemy"  # RPC提供商：alchemy 或 infura
  request_timeout: "30s"
  alchemy:
    api_key: "your-alchemy-api-key"
    ethereum: "https://eth-mainnet.g.alchemy.com/v2/"
    bsc: "https://bnb-mainnet.g.alchemy.com/v2/"
    polygon: "https://polygon-mainnet.g.alchemy.com/v2/"
    arbitrum: "https://arb-mainnet.g.alchemy.com/v2/"
  infura:
    api_key: "your-infura-api-key" 
    ethereum: "https://mainnet.infura.io/v3/"
    bsc: "https://bsc-dataseed.binance.org/"
    polygon: "https://polygon-mainnet.infura.io/v3/"
    arbitrum: "https://arbitrum-mainnet.infura.io/v3/"

# 价格服务配置
price:
  provider: "coingecko"
  api_key: ""  # 可选，用于提高请求限制
  base_url: "https://api.coingecko.com/api/v3"
  update_interval: "30s"
  request_timeout: "10s"
  cache_prefix: "price:"

# 资产服务配置
asset:
  update_interval: "30s"      # 资产更新间隔
  batch_size: 50              # 批量处理大小
  retry_attempts: 3           # 重试次数
  cache_ttl: "300s"          # 缓存生存时间
```

### 5. 启动服务

```bash
# 安装依赖
go mod tidy

# 启动服务
go run cmd/server/main.go
```

服务启动后：
- API服务: http://localhost:8080
- Swagger文档: http://localhost:8080/swagger/index.html
- 健康检查: http://localhost:8080/health

## 价格查询系统

### 支持的代币表（support_tokens）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint | 主键 |
| symbol | string | 代币符号（如BTC, ETH） |
| name | string | 代币名称 |
| coingecko_id | string | CoinGecko API ID |
| decimals | int | 代币精度 |
| is_active | boolean | 是否启用价格查询 |
| created_at | timestamp | 创建时间 |
| updated_at | timestamp | 更新时间 |

### 价格缓存机制

- **缓存键格式**: `price:{SYMBOL}` (如 `price:BTC`)
- **更新频率**: 30秒（可配置）
- **缓存过期**: 更新间隔的2倍
- **数据格式**: JSON格式的TokenPrice结构

### 添加新代币

```sql
INSERT INTO support_tokens (symbol, name, coingecko_id, decimals, is_active) 
VALUES ('NEW', 'New Token', 'new-token-id', 18, true);
```

## API接口

### 认证相关

- `POST /api/v1/auth/connect` - 钱包连接和用户注册
- `POST /api/v1/auth/refresh` - 刷新JWT令牌
- `GET /api/v1/auth/profile` - 获取用户资料

### 资产管理

- `GET /api/v1/assets` - 获取用户资产
- `POST /api/v1/assets/refresh` - 刷新用户指定链资产
- `POST /api/v1/assets/refresh-all` - 刷新所有用户资产（管理员接口）

#### 获取用户资产

```bash
# 获取所有链的资产
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/v1/assets

# 获取指定链的资产
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  "http://localhost:8080/api/v1/assets?chain_id=1"

# 强制刷新资产
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  "http://localhost:8080/api/v1/assets?force_refresh=true"
```

#### 刷新用户资产

```bash
# 刷新指定链的资产
curl -X POST \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"chain_id": 1}' \
  http://localhost:8080/api/v1/assets/refresh

# 刷新所有链的资产
curl -X POST \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://localhost:8080/api/v1/assets/refresh
```

#### 响应格式

```json
{
  "success": true,
  "data": {
    "total_usd_value": "1234.56789012",
    "chain_assets": [
      {
        "chain_info": {
          "chain_id": 1,
          "name": "Ethereum",
          "symbol": "ETH"
        },
        "total_usd_value": "1000.12345678",
        "assets": [
          {
            "token_info": {
              "symbol": "ETH",
              "name": "Ethereum",
              "decimals": 18
            },
            "balance": "1234567890123456789",
            "formatted_balance": "1.234567890123456789",
            "usd_value": "2500.12345678",
            "contract_address": "",
            "is_native": true,
            "last_updated": "2024-01-01T12:00:00Z"
          }
        ]
      }
    ]
  }
}
```

### 价格查询（内部服务）

价格查询服务作为后台服务运行，自动更新价格数据存储在Redis中：

```bash
# 查询特定代币价格
redis-cli GET "price:ETH"

# 查询所有价格
redis-cli KEYS "price:*"

# 价格数据格式
redis-cli GET "price:ETH" | jq '.'
```

## 开发指南

### 项目结构

```
timelocker-backend/
├── cmd/server/                    # 主程序入口
├── internal/
│   ├── api/                      # API处理器
│   │   ├── auth/                 # 认证相关接口
│   │   └── asset/                # 资产管理接口
│   ├── config/                   # 配置管理
│   ├── middleware/               # 中间件
│   ├── repository/               # 数据访问层
│   │   ├── user/                 # 用户数据仓库
│   │   ├── token/                # 代币数据仓库
│   │   ├── chain/                # 区块链数据仓库
│   │   ├── chaintoken/           # 链代币关联仓库
│   │   └── asset/                # 资产数据仓库
│   ├── service/                  # 业务逻辑层
│   │   ├── auth/                 # 认证服务
│   │   ├── price/                # 价格服务
│   │   └── asset/                # 资产服务
│   └── types/                    # 类型定义
├── pkg/
│   ├── blockchain/               # 区块链交互
│   ├── database/                 # 数据库连接
│   ├── crypto/                   # 加密相关
│   ├── logger/                   # 日志系统
│   └── utils/                    # 工具函数
├── scripts/                      # 数据库脚本
│   └── init_chains_and_tokens.sql
├── logs/                         # 日志文件
├── docs/                         # API文档
└── front-test/                   # 前端测试页面
```

### 扩展功能

#### 添加新的区块链支持

1. **数据库配置**：
```sql
-- 添加新的区块链
INSERT INTO support_chains (chain_id, name, symbol, rpc_provider, is_active) 
VALUES (25, 'Cronos', 'CRO', 'alchemy', true);
```

2. **RPC配置**：在 `config.yaml` 中添加新链的RPC端点
```yaml
rpc:
  alchemy:
    cronos: "https://cronos-mainnet.g.alchemy.com/v2/"
```

3. **代码更新**：在 `pkg/blockchain/rpc_client.go` 中添加新链的支持

#### 添加新的代币支持

1. **添加代币信息**：
```sql
INSERT INTO support_tokens (symbol, name, coingecko_id, decimals, is_active) 
VALUES ('CRO', 'Cronos', 'crypto-com-chain', 18, true);
```

2. **配置链代币关联**：
```sql
INSERT INTO chain_tokens (chain_id, token_id, contract_address, is_native, is_active)
SELECT 25, st.id, '', true, true
FROM support_tokens st WHERE st.symbol = 'CRO';
```

#### 扩展价格源

要添加新的价格源（如Binance API），需要：

1. 在 `config.yaml` 中修改 `price.provider`
2. 在 `price_service.go` 中的 `updatePrices` 方法添加新的case
3. 实现对应的价格获取方法

#### 数据库表结构修改

添加新字段时的最佳实践：

```sql
-- 添加新字段
ALTER TABLE users ADD COLUMN email VARCHAR(255);

-- 添加索引
CREATE INDEX idx_users_email ON users(email);

-- 更新现有记录
UPDATE users SET email = '' WHERE email IS NULL;
```

#### 性能优化

1. **数据库查询优化**：
```sql
-- 创建复合索引
CREATE INDEX idx_user_assets_composite ON user_assets(user_id, chain_id, last_updated);

-- 分区表（针对大数据量）
CREATE TABLE user_assets_partitioned (
    LIKE user_assets INCLUDING ALL
) PARTITION BY RANGE (created_at);
```

2. **缓存策略优化**：
- 使用Redis集群
- 实现分布式缓存
- 添加本地缓存层

### 日志系统

使用统一的日志格式：

```go
logger.Info("操作成功", "key1", value1, "key2", value2)
logger.Error("操作失败", err, "key1", value1)
logger.Debug("调试信息", "key1", value1)
```

## 部署

### Docker部署

```dockerfile
# Dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o main cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
COPY --from=builder /app/config.yaml .
CMD ["./main"]
```

### 环境变量

支持通过环境变量覆盖配置：

```bash
export SERVER_PORT=8080
export DATABASE_HOST=localhost
export REDIS_HOST=localhost
export PRICE_PROVIDER=coingecko
```

## 监控和维护

### 健康检查

```bash
curl http://localhost:8080/health
```

### 数据库状态检查

```bash
# 检查数据库连接
psql -U timelocker -d timelocker_db -c "SELECT version();"

# 检查表数量和大小
psql -U timelocker -d timelocker_db -c "
  SELECT 
    schemaname,
    tablename,
    attname,
    n_distinct,
    correlation
  FROM pg_stats 
  WHERE schemaname = 'public' 
  ORDER BY tablename;
"

# 检查索引使用情况
psql -U timelocker -d timelocker_db -c "
  SELECT 
    schemaname,
    tablename,
    indexname,
    idx_tup_read,
    idx_tup_fetch
  FROM pg_stat_user_indexes 
  ORDER BY idx_tup_read DESC;
"

# 检查数据库大小
psql -U timelocker -d timelocker_db -c "
  SELECT 
    pg_size_pretty(pg_database_size('timelocker_db')) as db_size;
"
```

### 资产服务状态

```bash
# 检查用户资产数量
psql -U timelocker -d timelocker_db -c "
  SELECT 
    COUNT(*) as total_users,
    COUNT(DISTINCT wallet_address) as unique_wallets
  FROM users;
"

# 检查资产更新状态
psql -U timelocker -d timelocker_db -c "
  SELECT 
    sc.name as chain_name,
    COUNT(*) as asset_count,
    MAX(ua.last_updated) as last_update
  FROM user_assets ua
  JOIN support_chains sc ON ua.chain_id = sc.id
  GROUP BY sc.name, sc.chain_id
  ORDER BY sc.chain_id;
"

# 检查资产总价值分布
psql -U timelocker -d timelocker_db -c "
  SELECT 
    CASE 
      WHEN usd_value = 0 THEN '0'
      WHEN usd_value < 10 THEN '< $10'
      WHEN usd_value < 100 THEN '$10-$100'
      WHEN usd_value < 1000 THEN '$100-$1K'
      ELSE '> $1K'
    END as value_range,
    COUNT(*) as count
  FROM user_assets
  GROUP BY 
    CASE 
      WHEN usd_value = 0 THEN '0'
      WHEN usd_value < 10 THEN '< $10'
      WHEN usd_value < 100 THEN '$10-$100'
      WHEN usd_value < 1000 THEN '$100-$1K'
      ELSE '> $1K'
    END
  ORDER BY count DESC;
"
```

### 价格服务状态

检查Redis中的价格数据：

```bash
# 检查价格更新时间
redis-cli GET "price:ETH" | jq '.last_updated'

# 统计缓存的代币数量
redis-cli KEYS "price:*" | wc -l

# 检查价格服务内存使用
redis-cli INFO memory

# 检查Redis连接状态
redis-cli INFO clients
```

### 日志查看

```bash
# 查看实时日志
tail -f logs/timelocker.log

# 查看错误日志
grep -i error logs/timelocker.log | tail -20

# 查看资产更新日志
grep -i "asset.*update" logs/timelocker.log | tail -10

# 查看RPC请求日志  
grep -i "rpc\|balance" logs/timelocker.log | tail -10
```

### 数据库维护

#### 定期维护任务

```bash
# 每日数据库统计更新
psql -U timelocker -d timelocker_db -c "ANALYZE;"

# 每周数据库清理
psql -U timelocker -d timelocker_db -c "VACUUM;"

# 每月完整清理
psql -U timelocker -d timelocker_db -c "VACUUM FULL;"
```

#### 数据清理

```bash
# 清理过期的用户资产记录（超过30天未更新）
psql -U timelocker -d timelocker_db -c "
  DELETE FROM user_assets 
  WHERE last_updated < NOW() - INTERVAL '30 days';
"

# 清理无效用户（无资产记录）
psql -U timelocker -d timelocker_db -c "
  DELETE FROM users 
  WHERE id NOT IN (SELECT DISTINCT user_id FROM user_assets);
"
```

## 故障排除

### 常见问题

#### 1. 数据库连接失败

```bash
# 检查PostgreSQL服务状态
sudo systemctl status postgresql

# 检查端口占用
sudo netstat -tulpn | grep 5432

# 测试数据库连接
psql -U timelocker -h localhost -d timelocker_db -c "SELECT 1;"
```

#### 2. RPC连接失败

```bash
# 检查RPC配置
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY

# 测试不同RPC提供商
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  https://mainnet.infura.io/v3/YOUR_API_KEY
```

#### 3. Redis连接问题

```bash
# 检查Redis服务
sudo systemctl status redis

# 测试Redis连接
redis-cli ping

# 检查Redis配置
redis-cli CONFIG GET "*"
```

#### 4. 资产更新异常

```bash
# 手动触发资产更新
curl -X POST \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/v1/assets/refresh-all

# 检查具体错误
grep -i "asset.*error\|rpc.*error" logs/timelocker.log | tail -10
```

## 开发指南

### 数据库开发规范

#### 1. 表命名规范
- 使用小写字母和下划线
- 表名使用复数形式（如 `users`, `user_assets`）
- 关联表使用 `table1_table2` 格式（如 `chain_tokens`）

#### 2. 字段命名规范
- 主键统一使用 `id`
- 外键使用 `table_id` 格式（如 `user_id`, `chain_id`）
- 时间字段使用 `created_at`, `updated_at`
- 布尔字段使用 `is_` 前缀（如 `is_active`, `is_native`）

#### 3. 数据类型规范
- 主键：`BIGSERIAL`
- 字符串：根据长度使用 `VARCHAR(n)`
- 金额：使用 `VARCHAR(78)` 存储大数字符串
- 价格：使用 `DECIMAL(20,8)` 保证精度
- 时间：使用 `TIMESTAMP WITH TIME ZONE`

#### 4. 索引策略
- 主键自动创建索引
- 外键字段创建索引
- 查询频繁的字段创建索引
- 复合查询创建复合索引

#### 5. 数据库迁移

创建新的迁移文件：
```sql
-- migrations/001_add_new_table.sql
CREATE TABLE new_table (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 创建索引
CREATE INDEX idx_new_table_name ON new_table(name);
```

回滚迁移：
```sql
-- migrations/001_add_new_table_down.sql
DROP INDEX IF EXISTS idx_new_table_name;
DROP TABLE IF EXISTS new_table;
```

### 代码开发规范

#### 1. Repository层开发

```go
// repository接口定义
type UserRepository interface {
    Create(ctx context.Context, user *types.User) error
    GetByWalletAddress(ctx context.Context, address string) (*types.User, error)
    Update(ctx context.Context, user *types.User) error
    Delete(ctx context.Context, id int64) error
}

// 实现示例
func (r *userRepository) Create(ctx context.Context, user *types.User) error {
    query := `
        INSERT INTO users (wallet_address, chain_id, nonce, signature, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `
    return r.db.QueryRowContext(ctx, query, 
        user.WalletAddress, user.ChainID, user.Nonce, 
        user.Signature, user.Status).Scan(
        &user.ID, &user.CreatedAt, &user.UpdatedAt)
}
```

#### 2. Service层开发

```go
// service接口定义
type AssetService interface {
    GetUserAssets(ctx context.Context, userID int64, chainID *int64) (*types.UserAssetsResponse, error)
    RefreshUserAssets(ctx context.Context, req *types.RefreshAssetsRequest) error
    RefreshAllUserAssets(ctx context.Context) error
}

// 事务处理示例
func (s *assetService) RefreshUserAssets(ctx context.Context, req *types.RefreshAssetsRequest) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 业务逻辑...
    
    return tx.Commit()
}
```

#### 3. 错误处理

```go
// 定义业务错误
var (
    ErrUserNotFound = errors.New("user not found")
    ErrChainNotSupported = errors.New("chain not supported")
    ErrInvalidWalletAddress = errors.New("invalid wallet address")
)

// 错误包装
func (r *userRepository) GetByWalletAddress(ctx context.Context, address string) (*types.User, error) {
    user := &types.User{}
    err := r.db.QueryRowContext(ctx, query, address).Scan(...)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, ErrUserNotFound
        }
        return nil, fmt.Errorf("failed to get user by wallet address: %w", err)
    }
    return user, nil
}
```

### 测试指南

#### 1. 数据库测试

```go
// 使用测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
    db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{})
    require.NoError(t, err)
    
    // 迁移测试表
    err = db.AutoMigrate(&types.User{}, &types.UserAsset{})
    require.NoError(t, err)
    
    return db
}

// 清理测试数据
func cleanupTestDB(t *testing.T, db *gorm.DB) {
    db.Exec("TRUNCATE users, user_assets CASCADE")
}
```

#### 2. 集成测试

```go
// 测试完整的业务流程
func TestAssetService_GetUserAssets(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    // 准备测试数据
    user := &types.User{
        WalletAddress: "0x1234...",
        ChainID: 1,
    }
    err := db.Create(user).Error
    require.NoError(t, err)
    
    // 执行测试
    service := NewAssetService(db, nil, nil)
    assets, err := service.GetUserAssets(context.Background(), user.ID, nil)
    
    // 验证结果
    assert.NoError(t, err)
    assert.NotNil(t, assets)
}
```

## 开发计划

### 短期计划 (1-2个月)
- [ ] 支持更多区块链（Avalanche, Fantom）
- [ ] 添加NFT资产查询
- [ ] 实现资产变化通知
- [ ] 添加资产历史记录

### 中期计划 (3-6个月) 
- [ ] 支持更多价格源（Binance, Coinbase等）
- [ ] 添加DeFi协议集成（Uniswap, Aave等）
- [ ] 实现跨链资产统计
- [ ] 添加价格预警功能

### 长期计划 (6个月+)
- [ ] 支持历史价格查询
- [ ] 实现消息队列（RabbitMQ/Kafka）
- [ ] 添加监控和指标系统
- [ ] 实现多语言支持
- [ ] 添加移动端API

## 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 许可证

MIT License
