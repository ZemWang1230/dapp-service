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

### 2. 数据库配置

```bash
# 创建数据库
createdb timelocker_db

# 创建用户
psql -c "CREATE USER timelocker WITH PASSWORD 'timelocker';"
psql -c "GRANT ALL PRIVILEGES ON DATABASE timelocker_db TO timelocker;"
```

### 3. 初始化代币数据

```bash
# 运行初始化脚本添加支持的代币
psql -U timelocker -d timelocker_db -f scripts/init_tokens.sql
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

price:
  provider: "coingecko"
  api_key: ""  # 可选，用于提高请求限制
  base_url: "https://api.coingecko.com/api/v3"
  update_interval: "30s"
  request_timeout: "10s"
  cache_prefix: "price:"
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

- `POST /api/v1/auth/connect` - 钱包连接
- `POST /api/v1/auth/refresh` - 刷新令牌
- `GET /api/v1/auth/profile` - 获取用户资料

### 价格查询（内部服务）

价格查询服务作为后台服务运行，不提供HTTP接口。价格数据存储在Redis中，可通过以下方式访问：

```bash
# 查询特定代币价格
redis-cli GET "price:BTC"

# 查询所有价格
redis-cli KEYS "price:*"
```

## 开发指南

### 项目结构

```
timelocker-backend/
├── cmd/server/          # 主程序入口
├── internal/
│   ├── api/            # API处理器
│   ├── config/         # 配置管理
│   ├── repository/     # 数据访问层
│   ├── service/        # 业务逻辑层
│   └── types/          # 类型定义
├── pkg/
│   ├── database/       # 数据库连接
│   ├── logger/         # 日志系统
│   └── utils/          # 工具函数
├── scripts/            # 脚本文件
└── docs/              # API文档
```

### 扩展价格源

要添加新的价格源，需要：

1. 在 `config.yaml` 中修改 `price.provider`
2. 在 `price_service.go` 中的 `updatePrices` 方法添加新的case
3. 实现对应的价格获取方法

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

### 价格服务状态

检查Redis中的价格数据：

```bash
# 检查价格更新时间
redis-cli GET "price:BTC" | jq '.last_updated'

# 统计缓存的代币数量
redis-cli KEYS "price:*" | wc -l
```

### 日志查看

```bash
tail -f logs/timelocker.log
```

## 开发计划

- [ ] 支持更多价格源（Binance, Coinbase等）
- [ ] 添加价格预警功能
- [ ] 支持历史价格查询
- [ ] 添加价格API接口
- [ ] 实现消息队列（RabbitMQ/Kafka）
- [ ] 添加监控和指标

## 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 许可证

MIT License
