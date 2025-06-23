# TimeLocker 数据库开发文档

## 概述

TimeLocker 使用PostgreSQL作为主数据库，设计了一个支持多链资产管理的完整数据库架构。本文档详细介绍数据库的设计思路、建立过程、维护方法和开发最佳实践。

## 数据库架构设计

### 核心设计理念

1. **多链支持**：通过`support_chains`表支持多个区块链网络
2. **代币抽象**：通过`support_tokens`表统一管理不同代币信息  
3. **灵活关联**：通过`chain_tokens`表实现链与代币的多对多关联
4. **用户资产**：通过`user_assets`表存储用户在各链上的代币余额
5. **数据一致性**：使用外键约束确保数据完整性

### 数据库表结构详解

#### 1. 用户表 (users)

**用途**：存储钱包用户信息和认证数据

```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,                        -- 用户ID，自增主键
    wallet_address VARCHAR(42) NOT NULL UNIQUE,     -- 钱包地址，以太坊格式
    chain_id INTEGER NOT NULL,                      -- 用户主要使用的链ID
    nonce VARCHAR(255) NOT NULL,                    -- 用于签名验证的随机数
    signature VARCHAR(132),                         -- 钱包签名数据
    status VARCHAR(50) DEFAULT 'active',            -- 用户状态：active, inactive, banned
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

**索引**：
```sql
CREATE INDEX idx_users_wallet_address ON users(wallet_address);
CREATE INDEX idx_users_chain_id ON users(chain_id);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_created_at ON users(created_at);
```

**设计说明**：
- `wallet_address`：42字符长度适合以太坊地址格式(0x + 40个十六进制)
- `chain_id`：记录用户初次连接时的链ID，用于默认显示
- `nonce`：每次登录时生成新的随机数，防止重放攻击
- `signature`：132字符长度适合以太坊签名格式(0x + 130个十六进制)

#### 2. 支持的区块链表 (support_chains)

**用途**：配置系统支持的区块链网络

```sql
CREATE TABLE support_chains (
    id BIGSERIAL PRIMARY KEY,                        -- 内部ID
    chain_id INTEGER NOT NULL UNIQUE,               -- 区块链网络ID（如以太坊为1）
    name VARCHAR(100) NOT NULL,                     -- 区块链名称
    symbol VARCHAR(10) NOT NULL,                    -- 原生代币符号
    rpc_provider VARCHAR(50) DEFAULT 'alchemy',     -- RPC提供商
    is_active BOOLEAN DEFAULT true,                 -- 是否启用
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

**索引**：
```sql
CREATE INDEX idx_support_chains_chain_id ON support_chains(chain_id);
CREATE INDEX idx_support_chains_is_active ON support_chains(is_active);
```

**设计说明**：
- `chain_id`：使用标准的区块链网络ID（EIP-155规范）
- `rpc_provider`：支持切换不同的RPC提供商（alchemy, infura等）
- `is_active`：支持动态启用/禁用特定链

#### 3. 支持的代币表 (support_tokens)

**用途**：管理系统支持的所有代币信息

```sql
CREATE TABLE support_tokens (
    id BIGSERIAL PRIMARY KEY,                        -- 代币内部ID
    symbol VARCHAR(20) NOT NULL UNIQUE,             -- 代币符号（如ETH, USDC）
    name VARCHAR(100) NOT NULL,                     -- 代币全名
    coingecko_id VARCHAR(100) NOT NULL UNIQUE,      -- CoinGecko API ID
    decimals INTEGER NOT NULL DEFAULT 18,           -- 代币精度（小数位数）
    is_active BOOLEAN DEFAULT true,                 -- 是否启用价格查询
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

**索引**：
```sql
CREATE INDEX idx_support_tokens_symbol ON support_tokens(symbol);
CREATE INDEX idx_support_tokens_coingecko_id ON support_tokens(coingecko_id);
CREATE INDEX idx_support_tokens_is_active ON support_tokens(is_active);
```

**设计说明**：
- `symbol`：代币符号，用于显示和查询
- `coingecko_id`：与CoinGecko API对应，用于获取价格
- `decimals`：代币精度，用于余额计算和显示

#### 4. 链代币关联表 (chain_tokens)

**用途**：定义代币在不同链上的配置（合约地址等）

```sql
CREATE TABLE chain_tokens (
    id BIGSERIAL PRIMARY KEY,
    chain_id INTEGER NOT NULL,                      -- 引用support_chains.id
    token_id INTEGER NOT NULL,                      -- 引用support_tokens.id
    contract_address VARCHAR(42) DEFAULT '',        -- 代币合约地址（原生代币为空）
    is_native BOOLEAN DEFAULT false,                -- 是否为链的原生代币
    is_active BOOLEAN DEFAULT true,                 -- 是否启用
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(chain_id, token_id),                     -- 联合唯一约束
    FOREIGN KEY (chain_id) REFERENCES support_chains(id) ON DELETE CASCADE,
    FOREIGN KEY (token_id) REFERENCES support_tokens(id) ON DELETE CASCADE
);
```

**索引**：
```sql
CREATE INDEX idx_chain_tokens_chain_id ON chain_tokens(chain_id);
CREATE INDEX idx_chain_tokens_token_id ON chain_tokens(token_id);
CREATE INDEX idx_chain_tokens_contract_address ON chain_tokens(contract_address);
CREATE INDEX idx_chain_tokens_is_active ON chain_tokens(is_active);
```

**设计说明**：
- `contract_address`：ERC-20代币的合约地址，原生代币为空字符串
- `is_native`：区分原生代币（如ETH）和ERC-20代币
- 联合唯一约束：确保同一个代币在同一条链上只有一个配置

#### 5. 用户资产表 (user_assets)

**用途**：存储用户在各链上的代币余额和价值

```sql
CREATE TABLE user_assets (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,                       -- 引用users.id
    wallet_address VARCHAR(42) NOT NULL,            -- 冗余存储，提高查询性能
    chain_id INTEGER NOT NULL,                      -- 引用support_chains.id  
    token_id INTEGER NOT NULL,                      -- 引用support_tokens.id
    balance VARCHAR(78) NOT NULL DEFAULT '0',       -- 余额（大数字符串）
    usd_value DECIMAL(20,8) DEFAULT 0,             -- USD价值
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(), -- 最后更新时间
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, chain_id, token_id),           -- 用户在特定链上的特定代币唯一
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (chain_id) REFERENCES support_chains(id) ON DELETE CASCADE,
    FOREIGN KEY (token_id) REFERENCES support_tokens(id) ON DELETE CASCADE
);
```

**索引**：
```sql
CREATE INDEX idx_user_assets_user_id ON user_assets(user_id);
CREATE INDEX idx_user_assets_wallet_address ON user_assets(wallet_address);
CREATE INDEX idx_user_assets_chain_id ON user_assets(chain_id);
CREATE INDEX idx_user_assets_last_updated ON user_assets(last_updated);
-- 复合索引，优化常用查询
CREATE INDEX idx_user_assets_composite ON user_assets(user_id, chain_id, last_updated);
```

**设计说明**：
- `balance`：使用VARCHAR(78)存储大整数字符串，支持最大精度
- `usd_value`：使用DECIMAL(20,8)保证价格计算精度
- `wallet_address`：冗余存储，避免关联查询，提高性能
- `last_updated`：记录余额最后更新时间，用于判断是否需要刷新

### 数据库建立详细步骤

#### 第一步：创建数据库和用户

```bash
# 方法1：使用createdb命令
createdb -U postgres timelocker_db
createuser -U postgres -P timelocker

# 方法2：使用SQL命令
sudo -u postgres psql -c "CREATE DATABASE timelocker_db;"
sudo -u postgres psql -c "CREATE USER timelocker WITH PASSWORD 'timelocker';"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE timelocker_db TO timelocker;"
sudo -u postgres psql -c "GRANT CREATE ON SCHEMA public TO timelocker;"
```

#### 第二步：验证数据库连接

```bash
# 测试连接
psql -U timelocker -h localhost -d timelocker_db -c "SELECT version();"

# 应该看到PostgreSQL版本信息
```

#### 第三步：运行应用程序自动建表

```bash
# 启动应用程序，自动创建表和索引
go run cmd/server/main.go
```

程序启动时会自动执行：
1. 连接数据库
2. 运行`AutoMigrate`创建表结构
3. 运行`CreateIndexes`创建索引
4. 修复可能的约束冲突

#### 第四步：初始化基础数据

```bash
# 执行初始化脚本
psql -U timelocker -d timelocker_db -f scripts/init_chains_and_tokens.sql
```

初始化脚本包含：
- 4个主流区块链配置
- 8个常用代币信息  
- 各链上代币的合约地址配置

#### 第五步：验证数据库建立

```bash
# 连接数据库
psql -U timelocker -d timelocker_db

# 检查表结构
\dt

# 检查数据
SELECT * FROM support_chains;
SELECT * FROM support_tokens;
SELECT 
    sc.name AS chain_name,
    st.symbol AS token_symbol,
    ct.contract_address,
    ct.is_native
FROM chain_tokens ct
JOIN support_chains sc ON ct.chain_id = sc.id
JOIN support_tokens st ON ct.token_id = st.id
ORDER BY sc.chain_id, st.symbol;
```

### 数据库维护指南

#### 日常维护任务

**每日任务**：
```bash
# 更新数据库统计信息
psql -U timelocker -d timelocker_db -c "ANALYZE;"

# 检查数据库大小
psql -U timelocker -d timelocker_db -c "
    SELECT 
        pg_size_pretty(pg_database_size('timelocker_db')) as database_size,
        pg_size_pretty(pg_total_relation_size('user_assets')) as user_assets_size;
"
```

**每周任务**：
```bash
# 数据库清理
psql -U timelocker -d timelocker_db -c "VACUUM;"

# 检查索引使用情况
psql -U timelocker -d timelocker_db -c "
    SELECT 
        schemaname,
        tablename,
        indexname,
        idx_tup_read,
        idx_tup_fetch,
        idx_tup_read/GREATEST(idx_tup_fetch, 1) as ratio
    FROM pg_stat_user_indexes 
    WHERE idx_tup_read > 0
    ORDER BY ratio DESC;
"
```

**每月任务**：
```bash
# 完整清理（需要停止应用程序）
psql -U timelocker -d timelocker_db -c "VACUUM FULL;"

# 重建索引（如果必要）
psql -U timelocker -d timelocker_db -c "REINDEX DATABASE timelocker_db;"
```

#### 数据清理策略

**清理过期数据**：
```sql
-- 清理30天未更新的资产记录
DELETE FROM user_assets 
WHERE last_updated < NOW() - INTERVAL '30 days';

-- 清理无效用户（无资产记录）
DELETE FROM users 
WHERE id NOT IN (SELECT DISTINCT user_id FROM user_assets);

-- 清理测试数据（如果有）
DELETE FROM users WHERE wallet_address LIKE '0x0000%';
```

**数据归档**：
```sql
-- 创建历史表
CREATE TABLE user_assets_history (
    LIKE user_assets INCLUDING ALL
);

-- 迁移历史数据
INSERT INTO user_assets_history 
SELECT * FROM user_assets 
WHERE last_updated < NOW() - INTERVAL '90 days';

-- 删除已归档的数据
DELETE FROM user_assets 
WHERE last_updated < NOW() - INTERVAL '90 days';
```

#### 备份和恢复

**自动备份脚本**：
```bash
#!/bin/bash
# backup_timelocker.sh

BACKUP_DIR="/var/backups/timelocker"
DATE=$(date +%Y%m%d_%H%M%S)
DB_NAME="timelocker_db"
DB_USER="timelocker"

# 创建备份目录
mkdir -p $BACKUP_DIR

# 完整备份
pg_dump -U $DB_USER -h localhost $DB_NAME > $BACKUP_DIR/full_backup_$DATE.sql

# 只备份数据
pg_dump -U $DB_USER -h localhost --data-only $DB_NAME > $BACKUP_DIR/data_backup_$DATE.sql

# 压缩备份
gzip $BACKUP_DIR/full_backup_$DATE.sql
gzip $BACKUP_DIR/data_backup_$DATE.sql

# 清理7天前的备份
find $BACKUP_DIR -name "*.gz" -mtime +7 -delete

echo "Backup completed at $(date)"
```

**恢复数据库**：
```bash
# 恢复完整数据库
gunzip -c full_backup_20240101_120000.sql.gz | psql -U timelocker -d timelocker_db

# 只恢复数据
gunzip -c data_backup_20240101_120000.sql.gz | psql -U timelocker -d timelocker_db
```

### 性能优化指南

#### 查询优化

**识别慢查询**：
```sql
-- 启用慢查询日志
ALTER SYSTEM SET log_min_duration_statement = 1000; -- 记录超过1秒的查询
SELECT pg_reload_conf();

-- 查看当前活跃查询
SELECT 
    pid,
    now() - pg_stat_activity.query_start AS duration,
    query,
    state
FROM pg_stat_activity
WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';
```

**优化查询示例**：
```sql
-- 原始查询（慢）
SELECT u.wallet_address, ua.balance, st.symbol
FROM users u
JOIN user_assets ua ON u.id = ua.user_id
JOIN support_tokens st ON ua.token_id = st.id
WHERE u.wallet_address = '0x1234...';

-- 优化后（使用冗余字段）
SELECT wallet_address, balance, st.symbol
FROM user_assets ua
JOIN support_tokens st ON ua.token_id = st.id
WHERE ua.wallet_address = '0x1234...';
```

#### 索引优化

**分析索引使用**：
```sql
-- 查看未使用的索引
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
WHERE idx_tup_read = 0 AND idx_tup_fetch = 0;

-- 查看重复索引
SELECT 
    pg_size_pretty(sum(pg_relation_size(idx))::bigint) as size,
    (array_agg(idx))[1] as idx1, 
    (array_agg(idx))[2] as idx2,
    (array_agg(idx))[3] as idx3,
    (array_agg(idx))[4] as idx4
FROM (
    SELECT indexrelid::regclass as idx, 
           (indrelid::regclass)::text as table_name, 
           regexp_split_to_array(indkey::text, ' ') as key, 
           array_length(regexp_split_to_array(indkey::text, ' '), 1) as nkeys
    FROM pg_index
) sub
GROUP BY table_name, key, nkeys 
HAVING count(*) > 1;
```

**创建合适的索引**：
```sql
-- 针对常用查询创建复合索引
CREATE INDEX idx_user_assets_query ON user_assets(user_id, chain_id, token_id, last_updated);

-- 针对范围查询创建索引
CREATE INDEX idx_user_assets_value_range ON user_assets(usd_value) WHERE usd_value > 0;

-- 部分索引，节省空间
CREATE INDEX idx_active_users ON users(wallet_address) WHERE status = 'active';
```

#### 数据库连接池优化

**GORM连接池配置**：
```go
// 在pkg/database/postgres.go中
sqlDB.SetMaxIdleConns(10)    // 空闲连接数
sqlDB.SetMaxOpenConns(100)   // 最大连接数
sqlDB.SetConnMaxLifetime(time.Hour) // 连接最大生存时间
```

**监控连接使用**：
```sql
-- 查看当前连接数
SELECT 
    count(*) as total_connections,
    count(*) FILTER (WHERE state = 'active') as active_connections,
    count(*) FILTER (WHERE state = 'idle') as idle_connections
FROM pg_stat_activity;

-- 查看连接详情
SELECT 
    pid,
    usename,
    application_name,
    client_addr,
    state,
    query_start,
    state_change
FROM pg_stat_activity
WHERE datname = 'timelocker_db';
```

### 开发最佳实践

#### 数据模型设计原则

1. **规范化与反规范化平衡**
   - 核心数据保持规范化（3NF）
   - 查询频繁的字段适当冗余
   - 使用外键约束保证数据完整性

2. **字段类型选择**
   - 金融数据使用精确类型（DECIMAL）
   - 大整数使用字符串存储（避免精度丢失）
   - 时间戳统一使用TIMESTAMP WITH TIME ZONE

3. **索引设计策略**
   - 查询条件字段创建索引
   - 复合查询创建复合索引
   - 避免过多索引影响写入性能

#### 代码开发规范

**Repository模式**：
```go
type UserAssetRepository interface {
    // 基础CRUD
    Create(ctx context.Context, asset *types.UserAsset) error
    GetByUserAndChain(ctx context.Context, userID int64, chainID *int64) ([]*types.UserAsset, error)
    Update(ctx context.Context, asset *types.UserAsset) error
    Delete(ctx context.Context, id int64) error
    
    // 批量操作
    BatchCreate(ctx context.Context, assets []*types.UserAsset) error
    BatchUpdate(ctx context.Context, assets []*types.UserAsset) error
    
    // 复杂查询
    GetUserTotalValue(ctx context.Context, userID int64) (decimal.Decimal, error)
    GetTopHolders(ctx context.Context, tokenID int64, limit int) ([]*types.UserAsset, error)
}
```

**事务处理**：
```go
func (s *AssetService) RefreshUserAssets(ctx context.Context, userID int64) error {
    return s.db.Transaction(func(tx *gorm.DB) error {
        // 在事务中执行多个操作
        if err := s.assetRepo.DeleteByUser(ctx, tx, userID); err != nil {
            return err
        }
        
        newAssets, err := s.fetchAssetsFromBlockchain(ctx, userID)
        if err != nil {
            return err
        }
        
        return s.assetRepo.BatchCreate(ctx, tx, newAssets)
    })
}
```

**错误处理**：
```go
// 定义业务级错误
var (
    ErrUserAssetNotFound = errors.New("user asset not found")
    ErrInvalidChainID = errors.New("invalid chain ID")
    ErrBalanceUpdateFailed = errors.New("balance update failed")
)

// 包装数据库错误
func (r *userAssetRepository) GetByID(ctx context.Context, id int64) (*types.UserAsset, error) {
    var asset types.UserAsset
    err := r.db.WithContext(ctx).First(&asset, id).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, ErrUserAssetNotFound
        }
        return nil, fmt.Errorf("failed to get user asset: %w", err)
    }
    return &asset, nil
}
```

### 监控和告警

#### 关键指标监控

**数据库性能指标**：
```sql
-- 数据库大小增长
SELECT 
    pg_size_pretty(pg_database_size('timelocker_db')) as current_size,
    pg_size_pretty(pg_database_size('timelocker_db') - 
        lag(pg_database_size('timelocker_db')) OVER (ORDER BY now())) as growth;

-- 表增长速度
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size,
    pg_stat_get_tuples_inserted(c.oid) as inserts,
    pg_stat_get_tuples_updated(c.oid) as updates,
    pg_stat_get_tuples_deleted(c.oid) as deletes
FROM pg_tables pt
JOIN pg_class c ON c.relname = pt.tablename
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

**业务指标监控**：
```sql
-- 活跃用户数
SELECT 
    date_trunc('day', last_updated) as date,
    count(DISTINCT user_id) as active_users
FROM user_assets
WHERE last_updated >= NOW() - INTERVAL '7 days'
GROUP BY date_trunc('day', last_updated)
ORDER BY date;

-- 资产更新频率
SELECT 
    sc.name as chain_name,
    count(*) as total_assets,
    count(*) FILTER (WHERE last_updated >= NOW() - INTERVAL '1 hour') as recent_updates,
    avg(extract(epoch from (NOW() - last_updated))/60) as avg_minutes_since_update
FROM user_assets ua
JOIN support_chains sc ON ua.chain_id = sc.id
GROUP BY sc.name
ORDER BY avg_minutes_since_update;
```

#### 告警规则

**数据库告警**：
- 连接数超过80%
- 数据库大小增长超过预期
- 慢查询数量增加
- 锁等待时间过长

**业务告警**：
- 资产更新失败率过高
- RPC请求失败率过高
- 用户资产数据过期
- 价格数据更新异常

## 总结

TimeLocker的数据库设计采用了现代化的多层架构，通过合理的表结构设计、索引优化和维护策略，确保了系统的高性能和高可用性。开发人员应当遵循本文档中的最佳实践，确保数据库的稳定运行和持续优化。

关键要点：
1. 使用标准化的表结构和命名规范
2. 通过索引和查询优化提高性能
3. 实施定期维护和监控策略
4. 遵循事务处理和错误处理最佳实践
5. 持续监控数据库健康状态和业务指标 