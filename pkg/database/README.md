# TimeLocker 数据库设计与部署指南

## 数据库关联设计

### 核心结构体关联

根据要求，系统保持以下核心结构体不变，重新设计了数据库关联关系：

1. **User (users表)** - 用户表
   - `user_id` 是唯一主键
   - `wallet_address` 是唯一索引
   - `chain_id` 存储用户当前使用的链ID

2. **SupportToken (support_tokens表)** - 支持的代币表
   - `token_id` 是唯一主键
   - 存储代币基本信息

3. **SupportChain (support_chains表)** - 支持的区块链表
   - `chain_id` 存储标准区块链网络ID (如以太坊为1)

4. **ChainToken (chain_tokens表)** - 链代币关联表
   - `chain_id` 关联 support_chains 表
   - `token_id` 关联 support_tokens 表
   - 存储合约地址和是否为原生代币等信息

5. **UserAsset (user_assets表)** - 用户资产表
   - `user_id` 关联 users 表
   - `token_id` 关联 support_tokens 表
   - `chain_id` 存储链ID
   - 唯一约束：`(user_id, chain_id, token_id)` 确保不重复

### 关键约束设计

```sql
-- 用户表：钱包地址唯一
users.wallet_address UNIQUE

-- 用户资产表：用户+链+代币组合唯一
user_assets(user_id, chain_id, token_id) UNIQUE

-- 链代币关联表：链+代币组合唯一  
chain_tokens(chain_id, token_id) UNIQUE
```

## 系统逻辑改进

### 1. 删除后台定时更新
- **移除**：后台定时刷新所有用户余额的逻辑
- **新逻辑**：仅在用户连接钱包或手动刷新时更新资产

### 2. 钱包连接时的资产更新
- 用户连接钱包时，异步更新该链上的资产信息
- 切换链时，将用户当作该链上的用户来处理

### 3. 余额刷新机制
- 使用 PostgreSQL 的 `ON CONFLICT` 语法实现 UPSERT
- 更新 user_assets 表而不是插入新记录
- 确保数据一致性和避免重复

## 部署指南

### 快速部署

1. **创建数据库**
```bash
# 连接到PostgreSQL
psql -U postgres -h localhost

# 创建数据库和用户
CREATE DATABASE timelocker_db;
CREATE USER timelocker WITH PASSWORD 'timelocker_password';
GRANT ALL PRIVILEGES ON DATABASE timelocker_db TO timelocker;
\q
```

2. **初始化数据库结构和数据**
```bash
# 方法1: 使用初始化脚本（推荐）
cd pkg/database
chmod +x init_db.sh
./init_db.sh timelocker_db timelocker timelocker_password localhost 5432

# 方法2: 手动执行SQL
psql -U timelocker -d timelocker_db -f init.sql
```

3. **启动服务**
```bash
# 确保配置文件正确
# 启动后端服务
go run cmd/server/main.go
```

### 验证部署

```sql
-- 检查表是否创建成功
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public';

-- 检查初始数据
SELECT * FROM support_chains;
SELECT * FROM support_tokens;
SELECT count(*) as chain_token_configs FROM chain_tokens;
```

## 数据库维护

### 清理重复数据（如果需要）
系统现在使用唯一约束防止重复，但如果需要清理历史数据：

```sql
-- 清理重复的用户资产（保留最新的）
WITH duplicate_assets AS (
    SELECT id, 
           ROW_NUMBER() OVER (
               PARTITION BY user_id, chain_id, token_id 
               ORDER BY last_updated DESC, id DESC
           ) as rn
    FROM user_assets
)
DELETE FROM user_assets 
WHERE id IN (SELECT id FROM duplicate_assets WHERE rn > 1);
```

### 性能优化

主要索引已通过 SQL 脚本创建：
- `users(wallet_address)` - 用户查询
- `user_assets(user_id, chain_id, token_id)` - 复合查询
- `chain_tokens(chain_id, token_id)` - 链代币查询

## 迁移说明

### 从旧版本迁移

如果从旧版本迁移，建议：

1. 备份现有数据
2. 执行新的初始化脚本
3. 迁移用户数据（如果需要保留）
4. 验证数据完整性

### 注意事项

- 确保 PostgreSQL 版本支持 `ON CONFLICT` 语法 (9.5+)
- 用户连接钱包时会触发资产更新，可能增加RPC调用
- 资产数据现在通过唯一约束保证一致性
- 删除了复杂的约束处理逻辑，简化了代码维护 