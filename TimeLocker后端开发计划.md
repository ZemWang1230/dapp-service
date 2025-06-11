# TimeLocker 后端开发计划

## 项目概述

TimeLocker 是一个去中心化的智能合约时间锁管理平台，允许用户创建、管理和监控时间延迟的区块链交易。该项目支持多链操作，包括以太坊、Arbitrum、BSC等主流区块链网络。

## 核心功能分析

### 1. 用户认证与钱包管理
**功能描述**: 支持用户通过Web3钱包连接进行身份验证
**核心任务**:
- 钱包地址验证和签名认证
- 用户会话管理
- 多链钱包地址关联
- 用户偏好设置存储

### 2. Timelock合约管理
**功能描述**: 创建、导入和管理Timelock智能合约
**核心任务**:
- 支持Compound和OpenZeppelin两种Timelock标准
- 合约部署和配置
- 合约参数管理（minDelay、角色权限）
- 合约状态监控

### 3. 交易调度与执行
**功能描述**: 创建延时交易并管理其生命周期
**核心任务**:
- 交易提案创建和编码
- 交易状态跟踪（待处理、执行中、已完成、已取消）
- 交易参数验证
- 批量交易支持

### 4. 多链资产监控
**功能描述**: 跨链资产追踪和价值计算
**核心任务**:
- 多链余额查询
- 资产价格获取和计算
- 历史数据统计
- 实时数据更新

### 5. 通知系统
**功能描述**: 邮件和其他形式的事件通知
**核心任务**:
- 邮件通知配置
- 事件触发器设置
- 通知模板管理
- 验证码系统

### 6. ABI管理
**功能描述**: 智能合约ABI的存储和管理
**核心任务**:
- ABI解析和验证
- 合约接口管理
- 函数调用编码
- ABI版本控制

### 7. 日志系统
**功能描述**: 系统操作日志记录、审计追踪和错误监控
**核心任务**:
- 用户操作日志记录
- 系统错误和异常追踪
- 区块链交互日志
- 安全审计日志
- 性能监控日志
- 日志查询和分析
- 日志归档和清理

## 技术架构设计

### 系统架构
```
Frontend (React/Vue) 
    ↓ HTTP/WebSocket
Backend API Gateway (Go Gin/Echo)
    ↓
┌─ Auth Service ─┐  ┌─ Blockchain Service ─┐  ┌─ Notification Service ─┐  ┌─ Logging Service ─┐
│  - JWT         │  │  - Web3 Integration  │  │  - Email Service      │  │  - Operation Logs  │
│  - Wallet Auth │  │  - Contract Manager  │  │  - SMS Service        │  │  - Error Tracking  │
│  - Session Mgmt│  │  - Transaction Pool  │  │  - Push Notifications │  │  - Audit Trail    │
└────────────────┘  └────────────────────────┘  └─────────────────────┘  └────────────────────┘
    ↓                           ↓                           ↓
┌─ Database Layer ─────────────────────────────────────────────────────┐
│  - PostgreSQL (主数据)                                                │
│  - Redis (缓存/会话)                                                  │
│  - MongoDB (日志/事件)                                                │
└──────────────────────────────────────────────────────────────────────┘
    ↓
┌─ External Services ──────────────────────────────────────────────────┐
│  - Ethereum/Arbitrum/BSC Nodes                                      │
│  - Price APIs (CoinGecko/CoinMarketCap)                            │
│  - Email Service (SendGrid/AWS SES)                                 │
└──────────────────────────────────────────────────────────────────────┘
```

### 数据库设计

#### PostgreSQL 主数据库

**1. users 表**
```sql
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

**2. timelocks 表**
```sql
CREATE TABLE timelocks (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    contract_address VARCHAR(42) NOT NULL,
    chain_id INTEGER NOT NULL,
    contract_type VARCHAR(20) NOT NULL, -- 'compound' or 'openzeppelin'
    min_delay BIGINT NOT NULL,
    admin_address VARCHAR(42),
    proposers TEXT[], -- 提案者地址数组
    executors TEXT[], -- 执行者地址数组
    contract_name VARCHAR(100),
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(contract_address, chain_id)
);
```

**3. transactions 表**
```sql
CREATE TABLE transactions (
    id BIGSERIAL PRIMARY KEY,
    timelock_id BIGINT REFERENCES timelocks(id),
    user_id BIGINT REFERENCES users(id),
    target_address VARCHAR(42) NOT NULL,
    value DECIMAL(36,18) DEFAULT 0,
    function_signature VARCHAR(100),
    calldata TEXT,
    delay_until TIMESTAMP NOT NULL,
    tx_hash VARCHAR(66),
    proposal_tx_hash VARCHAR(66),
    execution_tx_hash VARCHAR(66),
    status VARCHAR(20) DEFAULT 'pending', -- pending, queued, executed, cancelled, expired
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    executed_at TIMESTAMP,
    args JSONB DEFAULT '{}',
    gas_limit BIGINT,
    gas_price DECIMAL(36,18)
);
```

**4. assets 表**
```sql
CREATE TABLE assets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    chain_id INTEGER NOT NULL,
    token_address VARCHAR(42),
    token_symbol VARCHAR(20),
    token_name VARCHAR(100),
    balance DECIMAL(36,18),
    decimals INTEGER DEFAULT 18,
    price_usd DECIMAL(18,8),
    last_updated TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, chain_id, token_address)
);
```

**5. notifications 表**
```sql
CREATE TABLE notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    notification_type VARCHAR(20) NOT NULL, -- email, sms, push
    destination VARCHAR(255) NOT NULL, -- email address, phone, device token
    timelock_ids BIGINT[], -- 监控的timelock合约
    events TEXT[], -- 监控的事件类型
    is_active BOOLEAN DEFAULT true,
    verification_code VARCHAR(10),
    verified_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);
```

**6. abis 表**
```sql
CREATE TABLE abis (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    contract_address VARCHAR(42),
    abi_json JSONB NOT NULL,
    chain_id INTEGER,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

**7. blockchain_events 表**
```sql
CREATE TABLE blockchain_events (
    id BIGSERIAL PRIMARY KEY,
    timelock_id BIGINT REFERENCES timelocks(id),
    transaction_id BIGINT REFERENCES transactions(id),
    event_type VARCHAR(50) NOT NULL,
    tx_hash VARCHAR(66) NOT NULL,
    block_number BIGINT NOT NULL,
    log_index INTEGER NOT NULL,
    event_data JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(tx_hash, log_index)
);
```

**8. system_logs 表**
```sql
CREATE TABLE system_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    log_level VARCHAR(10) NOT NULL, -- DEBUG, INFO, WARN, ERROR, FATAL
    log_type VARCHAR(30) NOT NULL, -- USER_ACTION, SYSTEM_ERROR, BLOCKCHAIN_INTERACTION, SECURITY_AUDIT
    module VARCHAR(50) NOT NULL, -- auth, timelock, transaction, notification, etc.
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50), -- timelock, transaction, user, etc.
    resource_id VARCHAR(100),
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(50),
    session_id VARCHAR(100),
    message TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    stack_trace TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    INDEX idx_system_logs_user_created (user_id, created_at),
    INDEX idx_system_logs_type_level (log_type, log_level),
    INDEX idx_system_logs_created (created_at),
    INDEX idx_system_logs_request (request_id)
);
```

**9. audit_trails 表**
```sql
CREATE TABLE audit_trails (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    entity_type VARCHAR(50) NOT NULL, -- timelock, transaction, user, notification
    entity_id BIGINT NOT NULL,
    action VARCHAR(50) NOT NULL, -- CREATE, UPDATE, DELETE, EXECUTE, CANCEL
    old_values JSONB,
    new_values JSONB,
    ip_address INET,
    user_agent TEXT,
    session_id VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW(),
    INDEX idx_audit_trails_entity (entity_type, entity_id),
    INDEX idx_audit_trails_user_created (user_id, created_at),
    INDEX idx_audit_trails_created (created_at)
);
```

#### Redis 缓存设计
```
- user_session:{wallet_address} -> session_data
- asset_price:{token_symbol} -> price_data
- timelock_cache:{contract_address}:{chain_id} -> contract_info
- tx_queue:{chain_id} -> pending_transactions
- notification_queue -> pending_notifications
- log_buffer:{log_type} -> buffered_logs (批量写入优化)
- rate_limit:{ip_address} -> request_count
```

#### MongoDB 日志存储设计
```javascript
// 操作日志集合
db.operation_logs.createIndex({ "created_at": 1 }, { expireAfterSeconds: 7776000 }) // 90天过期
db.operation_logs.createIndex({ "user_id": 1, "created_at": -1 })
db.operation_logs.createIndex({ "log_type": 1, "created_at": -1 })

// 错误日志集合
db.error_logs.createIndex({ "created_at": 1 }, { expireAfterSeconds: 15552000 }) // 180天过期
db.error_logs.createIndex({ "log_level": 1, "created_at": -1 })
db.error_logs.createIndex({ "module": 1, "created_at": -1 })

// 性能监控日志
db.performance_logs.createIndex({ "created_at": 1 }, { expireAfterSeconds: 2592000 }) // 30天过期
db.performance_logs.createIndex({ "endpoint": 1, "created_at": -1 })
```

## API 接口设计

### 1. 认证模块 (/api/v1/auth)

```go
// POST /api/v1/auth/wallet-connect
type WalletConnectRequest struct {
    WalletAddress string `json:"wallet_address" binding:"required"`
    Signature     string `json:"signature" binding:"required"`
    Message       string `json:"message" binding:"required"`
    ChainId       int    `json:"chain_id" binding:"required"`
}

// POST /api/v1/auth/refresh
type RefreshTokenRequest struct {
    RefreshToken string `json:"refresh_token" binding:"required"`
}

// GET /api/v1/auth/profile
type UserProfile struct {
    WalletAddress string                 `json:"wallet_address"`
    CreatedAt     time.Time             `json:"created_at"`
    LastLogin     time.Time             `json:"last_login"`
    Preferences   map[string]interface{} `json:"preferences"`
}
```

### 2. Timelock管理 (/api/v1/timelocks)

```go
// GET /api/v1/timelocks
type TimelocksListResponse struct {
    Timelocks []TimelockInfo `json:"timelocks"`
    Total     int            `json:"total"`
    Page      int            `json:"page"`
    PageSize  int            `json:"page_size"`
}

// POST /api/v1/timelocks
type CreateTimelockRequest struct {
    ChainId      int      `json:"chain_id" binding:"required"`
    ContractType string   `json:"contract_type" binding:"required"` // compound/openzeppelin
    MinDelay     int64    `json:"min_delay" binding:"required"`
    Proposers    []string `json:"proposers"`
    Executors    []string `json:"executors"`
    Admin        string   `json:"admin"`
    Name         string   `json:"name"`
    Description  string   `json:"description"`
}

// POST /api/v1/timelocks/import
type ImportTimelockRequest struct {
    ChainId         int    `json:"chain_id" binding:"required"`
    ContractAddress string `json:"contract_address" binding:"required"`
    TxHash          string `json:"tx_hash"`
    Name            string `json:"name"`
    Description     string `json:"description"`
}

// GET /api/v1/timelocks/{id}/verify
type VerifyTimelockResponse struct {
    IsValid    bool                   `json:"is_valid"`
    ChainId    int                    `json:"chain_id"`
    Address    string                 `json:"address"`
    MinDelay   int64                  `json:"min_delay"`
    Roles      map[string][]string    `json:"roles"`
    Parameters map[string]interface{} `json:"parameters"`
}
```

### 3. 交易管理 (/api/v1/transactions)

```go
// GET /api/v1/transactions
type TransactionsListRequest struct {
    Page       int      `form:"page" binding:"min=1"`
    PageSize   int      `form:"page_size" binding:"min=1,max=100"`
    Status     []string `form:"status"`
    ChainId    int      `form:"chain_id"`
    TimelockId int64    `form:"timelock_id"`
    Search     string   `form:"search"`
}

// POST /api/v1/transactions
type CreateTransactionRequest struct {
    TimelockId        int64                  `json:"timelock_id" binding:"required"`
    TargetAddress     string                 `json:"target_address" binding:"required"`
    Value             string                 `json:"value"`
    FunctionSignature string                 `json:"function_signature"`
    Args              map[string]interface{} `json:"args"`
    DelaySeconds      int64                  `json:"delay_seconds" binding:"required"`
    Description       string                 `json:"description"`
}

// PUT /api/v1/transactions/{id}/execute
type ExecuteTransactionRequest struct {
    GasLimit int64  `json:"gas_limit"`
    GasPrice string `json:"gas_price"`
}

// DELETE /api/v1/transactions/{id}
// 取消交易

// GET /api/v1/transactions/{id}/simulate
type SimulateTransactionResponse struct {
    IsValid      bool   `json:"is_valid"`
    EstimatedGas int64  `json:"estimated_gas"`
    Error        string `json:"error,omitempty"`
}
```

### 4. 资产管理 (/api/v1/assets)

```go
// GET /api/v1/assets
type AssetsResponse struct {
    TotalValue string      `json:"total_value_usd"`
    Assets     []AssetInfo `json:"assets"`
    Chains     []ChainInfo `json:"chains"`
}

type AssetInfo struct {
    ChainId      int    `json:"chain_id"`
    ChainName    string `json:"chain_name"`
    TokenAddress string `json:"token_address"`
    TokenSymbol  string `json:"token_symbol"`
    TokenName    string `json:"token_name"`
    Balance      string `json:"balance"`
    Decimals     int    `json:"decimals"`
    PriceUSD     string `json:"price_usd"`
    ValueUSD     string `json:"value_usd"`
}

// POST /api/v1/assets/refresh
// 刷新资产数据
```

### 5. 通知管理 (/api/v1/notifications)

```go
// GET /api/v1/notifications
type NotificationsResponse struct {
    EmailNotifications []EmailNotification `json:"email_notifications"`
    Settings           NotificationSettings `json:"settings"`
}

// POST /api/v1/notifications/email
type CreateEmailNotificationRequest struct {
    Email       string   `json:"email" binding:"required,email"`
    TimelockIds []int64  `json:"timelock_ids"`
    Events      []string `json:"events"`
    Description string   `json:"description"`
}

// POST /api/v1/notifications/email/verify
type VerifyEmailRequest struct {
    Email            string `json:"email" binding:"required"`
    VerificationCode string `json:"verification_code" binding:"required"`
}

// POST /api/v1/notifications/email/send-code
type SendVerificationCodeRequest struct {
    Email string `json:"email" binding:"required,email"`
}
```

### 6. ABI管理 (/api/v1/abis)

```go
// GET /api/v1/abis
type ABIsResponse struct {
    ABIs     []ABIInfo `json:"abis"`
    Total    int       `json:"total"`
    Page     int       `json:"page"`
    PageSize int       `json:"page_size"`
}

// POST /api/v1/abis
type CreateABIRequest struct {
    Name            string                   `json:"name" binding:"required"`
    ContractAddress string                   `json:"contract_address"`
    ABI             []map[string]interface{} `json:"abi" binding:"required"`
    ChainId         int                      `json:"chain_id"`
}

// GET /api/v1/abis/{id}/functions
type ABIFunctionsResponse struct {
    Functions []FunctionInfo `json:"functions"`
}

type FunctionInfo struct {
    Name      string        `json:"name"`
    Type      string        `json:"type"`
    Inputs    []ParameterInfo `json:"inputs"`
    Outputs   []ParameterInfo `json:"outputs"`
    Signature string        `json:"signature"`
}
```

### 7. 日志管理 (/api/v1/logs)

```go
// GET /api/v1/logs/system
type SystemLogsRequest struct {
    Page      int       `form:"page" binding:"min=1"`
    PageSize  int       `form:"page_size" binding:"min=1,max=100"`
    LogLevel  []string  `form:"log_level"` // DEBUG, INFO, WARN, ERROR, FATAL
    LogType   []string  `form:"log_type"`  // USER_ACTION, SYSTEM_ERROR, etc.
    Module    string    `form:"module"`
    UserId    int64     `form:"user_id"`
    StartTime time.Time `form:"start_time"`
    EndTime   time.Time `form:"end_time"`
    Search    string    `form:"search"`
}

type SystemLogsResponse struct {
    Logs       []SystemLogInfo `json:"logs"`
    Total      int             `json:"total"`
    Page       int             `json:"page"`
    PageSize   int             `json:"page_size"`
    Statistics LogStatistics   `json:"statistics"`
}

// GET /api/v1/logs/audit
type AuditTrailsRequest struct {
    Page       int       `form:"page" binding:"min=1"`
    PageSize   int       `form:"page_size" binding:"min=1,max=100"`
    EntityType string    `form:"entity_type"`
    EntityId   int64     `form:"entity_id"`
    Action     []string  `form:"action"`
    UserId     int64     `form:"user_id"`
    StartTime  time.Time `form:"start_time"`
    EndTime    time.Time `form:"end_time"`
}

type AuditTrailsResponse struct {
    Trails   []AuditTrailInfo `json:"trails"`
    Total    int              `json:"total"`
    Page     int              `json:"page"`
    PageSize int              `json:"page_size"`
}

// GET /api/v1/logs/statistics
type LogStatisticsResponse struct {
    ErrorRates      map[string]float64 `json:"error_rates"`      // 按模块统计错误率
    RequestCounts   map[string]int64   `json:"request_counts"`   // 按小时统计请求量
    ResponseTimes   map[string]float64 `json:"response_times"`   // 平均响应时间
    UserActivities  map[string]int64   `json:"user_activities"`  // 用户活跃度
    SystemHealth    SystemHealthInfo   `json:"system_health"`    // 系统健康状态
}

// POST /api/v1/logs/export
type ExportLogsRequest struct {
    LogType   string    `json:"log_type" binding:"required"`
    StartTime time.Time `json:"start_time" binding:"required"`
    EndTime   time.Time `json:"end_time" binding:"required"`
    Format    string    `json:"format"` // csv, json, excel
    Filters   LogFilter `json:"filters"`
}

type ExportLogsResponse struct {
    DownloadUrl string    `json:"download_url"`
    ExpiresAt   time.Time `json:"expires_at"`
    FileSize    int64     `json:"file_size"`
}
```

## 服务架构实现

### 目录结构
```
timelocker-backend/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── api/
│   │   ├── auth/
│   │   ├── timelock/
│   │   ├── transaction/
│   │   ├── asset/
│   │   ├── notification/
│   │   ├── abi/
│   │   └── logs/
│   ├── service/
│   │   ├── auth/
│   │   ├── blockchain/
│   │   ├── notification/
│   │   ├── price/
│   │   └── logging/
│   ├── repository/
│   │   ├── user/
│   │   ├── timelock/
│   │   ├── transaction/
│   │   ├── asset/
│   │   └── logs/
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── cors.go
│   │   ├── rate_limit.go
│   │   └── logging.go
│   ├── config/
│   └── types/
├── pkg/
│   ├── web3/
│   ├── crypto/
│   ├── email/
│   └── utils/
├── migrations/
├── docker/
├── scripts/
└── docs/
```

### 核心服务实现

#### 1. 区块链服务 (blockchain service)
```go
type BlockchainService interface {
    // 合约交互
    DeployTimelock(ctx context.Context, params DeployParams) (*common.Address, error)
    GetTimelockInfo(ctx context.Context, address common.Address, chainId int) (*TimelockInfo, error)
    
    // 交易管理
    ProposeTransaction(ctx context.Context, params ProposeParams) (string, error)
    ExecuteTransaction(ctx context.Context, params ExecuteParams) (string, error)
    CancelTransaction(ctx context.Context, params CancelParams) (string, error)
    
    // 事件监听
    SubscribeTimelockEvents(ctx context.Context, addresses []common.Address) (<-chan Event, error)
    
    // 余额查询
    GetTokenBalance(ctx context.Context, tokenAddr, walletAddr common.Address, chainId int) (*big.Int, error)
    GetETHBalance(ctx context.Context, walletAddr common.Address, chainId int) (*big.Int, error)
}
```

#### 2. 价格服务 (price service)
```go
type PriceService interface {
    GetTokenPrice(ctx context.Context, symbol string) (float64, error)
    GetTokenPrices(ctx context.Context, symbols []string) (map[string]float64, error)
    SubscribePriceUpdates(ctx context.Context, symbols []string) (<-chan PriceUpdate, error)
}
```

#### 3. 通知服务 (notification service)
```go
type NotificationService interface {
    SendEmail(ctx context.Context, to, subject, body string) error
    SendVerificationCode(ctx context.Context, email string) (string, error)
    VerifyCode(ctx context.Context, email, code string) error
    
    // 事件通知
    NotifyTransactionProposed(ctx context.Context, tx *Transaction) error
    NotifyTransactionExecuted(ctx context.Context, tx *Transaction) error
    NotifyTransactionCancelled(ctx context.Context, tx *Transaction) error
}
```

#### 4. 日志服务 (logging service)
```go
type LoggingService interface {
    // 基础日志记录
    LogInfo(ctx context.Context, module, action, message string, metadata map[string]interface{}) error
    LogError(ctx context.Context, module, action string, err error, metadata map[string]interface{}) error
    LogWarn(ctx context.Context, module, action, message string, metadata map[string]interface{}) error
    LogDebug(ctx context.Context, module, action, message string, metadata map[string]interface{}) error
    
    // 审计日志
    LogAudit(ctx context.Context, userID int64, entityType string, entityID int64, action string, oldValues, newValues interface{}) error
    
    // 用户操作日志
    LogUserAction(ctx context.Context, userID int64, action, resource string, metadata map[string]interface{}) error
    
    // 区块链交互日志
    LogBlockchainInteraction(ctx context.Context, chainID int, txHash, action string, metadata map[string]interface{}) error
    
    // 性能监控日志
    LogPerformance(ctx context.Context, endpoint string, method string, duration time.Duration, statusCode int) error
    
    // 安全事件日志
    LogSecurityEvent(ctx context.Context, eventType, description string, severity string, metadata map[string]interface{}) error
    
    // 日志查询
    GetSystemLogs(ctx context.Context, req *SystemLogsRequest) (*SystemLogsResponse, error)
    GetAuditTrails(ctx context.Context, req *AuditTrailsRequest) (*AuditTrailsResponse, error)
    GetLogStatistics(ctx context.Context, timeRange string) (*LogStatisticsResponse, error)
    
    // 日志导出
    ExportLogs(ctx context.Context, req *ExportLogsRequest) (*ExportLogsResponse, error)
    
    // 日志清理
    CleanupLogs(ctx context.Context, retentionDays int) error
}

// 日志级别定义
const (
    LogLevelDebug = "DEBUG"
    LogLevelInfo  = "INFO"
    LogLevelWarn  = "WARN"
    LogLevelError = "ERROR"
    LogLevelFatal = "FATAL"
)

// 日志类型定义
const (
    LogTypeUserAction           = "USER_ACTION"
    LogTypeSystemError          = "SYSTEM_ERROR"
    LogTypeBlockchainInteraction = "BLOCKCHAIN_INTERACTION"
    LogTypeSecurityAudit        = "SECURITY_AUDIT"
    LogTypePerformanceMonitor   = "PERFORMANCE_MONITOR"
    LogTypeApiAccess           = "API_ACCESS"
)
```

## 与前端交互规范

### 1. WebSocket 实时通信
```go
// 事件类型
const (
    EventTransactionStatusUpdate = "transaction_status_update"
    EventAssetBalanceUpdate     = "asset_balance_update"
    EventTimelockEventReceived  = "timelock_event_received"
    EventNotificationReceived   = "notification_received"
    EventSystemLogReceived      = "system_log_received"
    EventErrorOccurred          = "error_occurred"
)

// WebSocket 消息格式
type WSMessage struct {
    Type      string                 `json:"type"`
    Data      map[string]interface{} `json:"data"`
    Timestamp time.Time             `json:"timestamp"`
}
```

### 2. 错误处理标准
```go
type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   *APIError   `json:"error,omitempty"`
}

type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

// 错误代码规范
const (
    ErrCodeInvalidWallet     = "INVALID_WALLET"
    ErrCodeInsufficientFunds = "INSUFFICIENT_FUNDS"
    ErrCodeContractNotFound  = "CONTRACT_NOT_FOUND"
    ErrCodeTransactionFailed = "TRANSACTION_FAILED"
    ErrCodeLogAccessDenied   = "LOG_ACCESS_DENIED"
    ErrCodeLogExportFailed   = "LOG_EXPORT_FAILED"
    ErrCodeInvalidLogQuery   = "INVALID_LOG_QUERY"
    // ...
)
```

### 3. 分页和过滤标准
```go
type PaginationRequest struct {
    Page     int `form:"page" binding:"min=1"`
    PageSize int `form:"page_size" binding:"min=1,max=100"`
}

type PaginationResponse struct {
    Page       int `json:"page"`
    PageSize   int `json:"page_size"`
    Total      int `json:"total"`
    TotalPages int `json:"total_pages"`
}

type FilterRequest struct {
    Search    string            `form:"search"`
    Filters   map[string]string `form:"filters"`
    SortBy    string            `form:"sort_by"`
    SortOrder string            `form:"sort_order"` // asc, desc
}
```

## 开发计划和里程碑

### 第一阶段：基础架构 (Week 1-2)
- [ ] 项目初始化和依赖管理
- [ ] 数据库设计和迁移脚本
- [ ] 基础配置和环境管理
- [ ] 认证中间件和JWT实现
- [ ] API路由框架搭建

### 第二阶段：核心功能 (Week 3-5)
- [ ] 用户认证和钱包连接
- [ ] Web3集成和合约交互
- [ ] Timelock合约管理
- [ ] 基础交易CRUD操作
- [ ] 数据库操作层实现

### 第三阶段：高级功能 (Week 6-8)
- [ ] 多链资产监控
- [ ] 交易状态追踪
- [ ] 事件监听和处理
- [ ] 价格数据集成
- [ ] WebSocket实时通信

### 第四阶段：通知和扩展 (Week 9-10)
- [ ] 邮件通知系统
- [ ] ABI管理功能
- [ ] 日志系统实现
- [ ] 审计追踪功能
- [ ] 批量操作支持
- [ ] 性能优化和缓存

### 第五阶段：测试和部署 (Week 11-12)
- [ ] 单元测试和集成测试
- [ ] API文档生成
- [ ] Docker容器化
- [ ] CI/CD管道搭建
- [ ] 安全审计和漏洞扫描

## 技术选型建议

### 核心框架和库
- **Web框架**: Gin (高性能、轻量级)
- **数据库ORM**: GORM (功能丰富、社区活跃)
- **Web3库**: go-ethereum (官方支持)
- **缓存**: go-redis (Redis客户端)
- **配置管理**: Viper
- **日志**: Zap (高性能结构化日志)
- **验证**: go-playground/validator
- **文档数据库**: MongoDB Driver (日志存储)
- **任务队列**: Asynq (日志处理异步任务)

### 外部服务集成
- **价格API**: CoinGecko Free API
- **邮件服务**: SendGrid 或 AWS SES
- **区块链节点**: Infura 或 Alchemy
- **监控**: Prometheus + Grafana

### 安全考虑
- JWT Token 过期机制
- API Rate Limiting
- 输入参数验证和清理
- SQL注入防护
- CORS配置
- 私钥和敏感信息加密存储
- 日志数据脱敏处理
- 审计日志完整性保护
- 敏感操作强制日志记录
- 日志访问权限控制

## 部署架构

### Docker化部署
```yaml
# docker-compose.yml
version: '3.8'
services:
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - redis
  
  postgres:
    image: postgres:14
    environment:
      POSTGRES_DB: timelocker
      POSTGRES_USER: timelocker
      POSTGRES_PASSWORD: password
    volumes:
      - postgres_data:/var/lib/postgresql/data
  
  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
  
  mongodb:
    image: mongo:6
    environment:
      MONGO_INITDB_ROOT_USERNAME: timelocker
      MONGO_INITDB_ROOT_PASSWORD: password
      MONGO_INITDB_DATABASE: timelocker_logs
    volumes:
      - mongodb_data:/data/db

volumes:
  postgres_data:
  redis_data:
  mongodb_data:
```

### 监控和日志
- 应用指标监控 (Prometheus)
- 日志聚合 (ELK Stack)
- 错误追踪 (Sentry)
- 性能监控 (APM)

这个开发计划涵盖了TimeLocker项目的所有核心功能，按照优先级和依赖关系进行了合理的安排。建议采用敏捷开发方式，每个阶段结束后进行功能验证和代码审查。
