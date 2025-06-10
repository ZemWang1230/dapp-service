# TimeLocker Backend

TimeLocker 后端服务，提供去中心化时间锁管理平台的API服务。

## 功能特性

- ✅ **钱包认证**: 支持以太坊钱包签名认证
- ✅ **JWT令牌**: 访问令牌和刷新令牌机制
- ✅ **用户管理**: 自动用户创建和资料管理
- ✅ **多链支持**: 支持以太坊、Arbitrum、BSC等网络
- 🚧 **Timelock管理**: 智能合约时间锁管理
- 🚧 **交易调度**: 延时交易创建和执行
- 🚧 **资产监控**: 多链资产余额追踪

## 技术栈

- **后端框架**: Gin (Go)
- **数据库**: PostgreSQL + GORM
- **缓存**: Redis
- **区块链**: go-ethereum
- **认证**: JWT
- **配置**: Viper

## 快速开始

### 1. 环境要求

- Go 1.19+
- PostgreSQL 12+
- Redis 6+

### 2. 安装依赖

```bash
go mod download
```

### 3. 配置数据库

创建PostgreSQL数据库：

```sql
CREATE DATABASE timelocker;
CREATE USER timelocker WITH PASSWORD 'password';
GRANT ALL PRIVILEGES ON DATABASE timelocker TO timelocker;
```

### 4. 配置应用

复制配置文件并修改：

```bash
cp config.yaml.example config.yaml
# 编辑 config.yaml 设置你的数据库连接信息
```

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
  "wallet_address": "0x742d35Cc6bF34C7a14b8f6c8a63f8a12345F6789",
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
      "created_at": "2024-01-01T00:00:00Z",
      "last_login": "2024-01-01T00:00:00Z",
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
    "wallet_address": "0x742d35cc6bf34c7a14b8f6c8a63f8a12345f6789",
    "created_at": "2024-01-01T00:00:00Z",
    "last_login": "2024-01-01T00:00:00Z",
    "preferences": {}
  }
}
```

### 健康检查

**GET** `/health`

检查服务状态。

## 前端集成示例

### Web3.js 示例

```javascript
import Web3 from 'web3';

async function connectWallet() {
  // 1. 连接钱包
  const web3 = new Web3(window.ethereum);
  const accounts = await web3.eth.requestAccounts();
  const walletAddress = accounts[0];
  
  // 2. 生成签名消息
  const message = `TimeLocker Login Nonce: ${Date.now()}`;
  
  // 3. 请求用户签名
  const signature = await web3.eth.personal.sign(message, walletAddress);
  
  // 4. 发送认证请求
  const response = await fetch('/api/v1/auth/wallet-connect', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      wallet_address: walletAddress,
      signature: signature,
      message: message,
      chain_id: await web3.eth.getChainId()
    })
  });
  
  const result = await response.json();
  
  if (result.success) {
    // 保存令牌
    localStorage.setItem('access_token', result.data.access_token);
    localStorage.setItem('refresh_token', result.data.refresh_token);
    
    console.log('认证成功:', result.data.user);
  } else {
    console.error('认证失败:', result.error);
  }
}
```

### Ethers.js 示例

```javascript
import { ethers } from 'ethers';

async function connectWallet() {
  // 1. 连接钱包
  const provider = new ethers.providers.Web3Provider(window.ethereum);
  await provider.send("eth_requestAccounts", []);
  const signer = provider.getSigner();
  const walletAddress = await signer.getAddress();
  
  // 2. 生成签名消息
  const message = `TimeLocker Login Nonce: ${Date.now()}`;
  
  // 3. 请求用户签名
  const signature = await signer.signMessage(message);
  
  // 4. 发送认证请求
  const response = await fetch('/api/v1/auth/wallet-connect', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      wallet_address: walletAddress,
      signature: signature,
      message: message,
      chain_id: (await provider.getNetwork()).chainId
    })
  });
  
  const result = await response.json();
  
  if (result.success) {
    // 保存令牌
    localStorage.setItem('access_token', result.data.access_token);
    localStorage.setItem('refresh_token', result.data.refresh_token);
    
    console.log('认证成功:', result.data.user);
  } else {
    console.error('认证失败:', result.error);
  }
}
```

## 认证流程说明

### 1. 签名验证过程

1. **前端生成消息**: 创建包含随机nonce的消息
2. **用户签名**: 钱包对消息进行签名
3. **后端验证**: 使用以太坊签名恢复算法验证签名
4. **地址匹配**: 确认恢复的地址与声明的钱包地址一致

### 2. 用户管理

- **自动注册**: 首次认证时自动创建用户记录
- **地址标准化**: 统一使用小写格式存储钱包地址
- **登录追踪**: 记录用户最后登录时间

### 3. JWT令牌机制

- **访问令牌**: 有效期24小时，用于API访问认证
- **刷新令牌**: 有效期7天，用于获取新的访问令牌
- **安全措施**: 包含用户ID和钱包地址，防止令牌伪造

## 错误处理

### 常见错误码

- `INVALID_WALLET_ADDRESS`: 钱包地址格式无效
- `INVALID_SIGNATURE`: 签名验证失败
- `SIGNATURE_RECOVERY_FAILED`: 无法从签名中恢复地址
- `INVALID_TOKEN`: JWT令牌无效或过期
- `USER_NOT_FOUND`: 用户不存在

### 错误响应格式

```json
{
  "success": false,
  "error": {
    "code": "INVALID_SIGNATURE",
    "message": "Signature verification failed",
    "details": "Additional error details..."
  }
}
```

## 安全考虑

1. **签名验证**: 使用以太坊标准的消息签名和恢复算法
2. **地址验证**: 多重验证确保钱包地址真实性
3. **令牌安全**: JWT使用HMAC-SHA256算法签名
4. **CORS配置**: 生产环境需要配置适当的CORS策略
5. **HTTPS**: 生产环境必须使用HTTPS传输

## 开发计划

- [x] 基础钱包认证
- [x] JWT令牌管理
- [x] 用户资料管理
- [ ] Timelock合约管理
- [ ] 交易调度系统
- [ ] 多链资产监控
- [ ] 通知系统
- [ ] 日志审计

## 贡献指南

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 发起 Pull Request

## 许可证

MIT License 