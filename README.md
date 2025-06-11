# TimeLocker Backend

TimeLocker 后端服务，提供去中心化时间锁管理平台的API服务。

## 功能特性

- **钱包认证**: 支持以太坊钱包签名认证
- **JWT令牌**: 访问令牌和刷新令牌机制
- **用户管理**: 自动用户创建和资料管理
- **多链支持**: 支持以太坊、Arbitrum、BSC等网络
- **Timelock管理**: 智能合约时间锁管理
- **交易调度**: 延时交易创建和执行
- **资产监控**: 多链资产余额追踪

## 技术栈

- **后端框架**: Gin (Go)
- **数据库**: PostgreSQL + GORM
- **缓存**: Redis
- **区块链**: go-ethereum
- **认证**: JWT
- **配置**: Viper

## 快速开始

### 1. 环境要求

- Go 1.23.10
- PostgreSQL 14.18
- Redis 8.0.2

### 2. 安装依赖

```bash
go mod download
```

### 3. 配置数据库

创建PostgreSQL数据库：

```sql
CREATE USER timelocker WITH ENCRYPTED PASSWORD 'timelocker';
CREATE DATABASE timelocker_db OWNER timelocker;
GRANT ALL PRIVILEGES ON DATABASE timelocker_db TO timelocker;
```

数据库设计:

```sql
-- User表
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    wallet_address VARCHAR(42) UNIQUE NOT NULL,
    chain_id INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_login TIMESTAMP,
    preferences JSONB DEFAULT '{}',
    status INTEGER DEFAULT 1
);
```



### 4. 配置应用

配置文件:config.yaml

### 5. 启动服务

```bash
go run cmd/server/main.go
```

服务将在 `http://localhost:8080` 启动。

## API 文档

### 认证相关接口

#### 1. 钱包连接认证

**POST** `/api/v1/auth/wallet-connect`

通过钱包签名进行用户认证。前端需要先让用户签名一个消息，然后将签名结果发送到此接口。

**请求体**:

```json
{
  "wallet_address": "0x...",
  "signature": "0x...",
  "message": "TimeLocker Login Nonce: 1234567890",
  "chain_id": 1
}
```

**响应**:
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2024-01-01T00:00:00Z",
    "user": {
      "id": 1,
      "wallet_address": "0x742d35cc6bf34c7a14b8f6c8a63f8a12345f6789",
      "chain_id": 1,
      "created_at": "2025-01-01T00:00:00Z",
      "last_login": "2025-01-01T00:00:00Z",
      "preferences": {},
      "status": 1
    }
  }
}
```

#### 2. 刷新访问令牌

**POST** `/api/v1/auth/refresh`

使用刷新令牌获取新的访问令牌。

**请求体**:
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

#### 3. 获取用户资料

**GET** `/api/v1/auth/profile`

获取当前认证用户的资料信息。

**请求头**:
```
Authorization: Bearer <access_token>
```

**响应**:
```json
{
  "success": true,
  "data": {
    "wallet_address": "0x...",
    "chain_id": 1,
    "created_at": "2025-01-01T00:00:00Z",
    "last_login": "2025-01-01T00:00:00Z",
    "preferences": {}
  }
}
```

### 健康检查

**GET** `/health`

检查服务状态。
