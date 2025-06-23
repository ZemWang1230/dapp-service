# TimeLocker 后端开发计划

## 项目概述

TimeLocker是一个基于区块链的去中心化时间锁管理平台，为用户提供安全的资产锁定和定时释放服务。

## 已完成功能 ✅

### 1. 基础架构
- [x] 项目结构搭建 (Go + Gin)
- [x] 数据库设计 (PostgreSQL + GORM)
- [x] 配置管理 (Viper + YAML)
- [x] 日志系统 (Zap)
- [x] JWT认证系统
- [x] Swagger API文档

### 2. 用户管理系统
- [x] 钱包地址认证
- [x] 用户注册和登录
- [x] JWT令牌管理 (Access + Refresh)
- [x] 用户资料管理
- [x] 多链支持 (Chain ID)

### 3. 价格查询系统 ✅ **新完成**
- [x] 支持代币表设计 (support_tokens)
- [x] CoinGecko价格源集成
- [x] Redis缓存机制
- [x] 自动价格更新服务 (30秒间隔)
- [x] 价格数据API结构
- [x] 低耦合价格源设计 (支持扩展)
- [x] 异步更新队列 (基于定时器)
- [x] 价格缓存管理
- [x] 错误处理和重试机制
- [x] 配置化价格源管理

#### 价格系统技术细节
- **数据库**: support_tokens表存储支持的代币信息
- **缓存**: Redis存储实时价格，键格式为 `price:{SYMBOL}`
- **更新机制**: 每30秒从CoinGecko批量获取价格
- **容错性**: 单个代币失败不影响其他代币更新
- **扩展性**: 支持添加新价格源（Binance、Coinbase等）

## 正在开发功能 🚧

### 4. 资产管理系统
- [ ] 钱包资产查询
- [ ] 多链资产聚合
- [ ] 资产价值计算 (基于价格缓存)
- [ ] 历史资产记录

### 5. 时间锁管理
- [ ] 智能合约集成
- [ ] 时间锁创建
- [ ] 锁定资产管理
- [ ] 释放条件管理

## 计划开发功能 📋

### 6. 交易调度系统
- [ ] 延时交易创建
- [ ] 交易执行引擎
- [ ] 交易状态追踪
- [ ] 失败重试机制

### 7. 通知系统
- [ ] 邮件通知
- [ ] 推送通知
- [ ] Webhook通知
- [ ] 通知规则配置

### 8. 风险管理
- [ ] 资产风险评估
- [ ] 市场波动监控
- [ ] 异常交易检测
- [ ] 紧急停止机制

### 9. 数据分析
- [ ] 用户行为分析
- [ ] 资产趋势分析
- [ ] 交易统计报表
- [ ] 性能监控看板

### 10. 高级功能
- [ ] 多签名支持
- [ ] 跨链资产管理
- [ ] DeFi协议集成
- [ ] NFT资产支持

## 技术优化计划 🔧

### 性能优化
- [ ] 数据库查询优化
- [ ] Redis集群部署
- [ ] 服务监控完善
- [ ] 负载均衡配置

### 安全加固
- [ ] API访问限制
- [ ] 敏感数据加密
- [ ] 审计日志完善
- [ ] 渗透测试

### 运维完善
- [ ] Docker容器化
- [ ] CI/CD流水线
- [ ] 自动化测试
- [ ] 监控告警系统

## 价格系统扩展计划 💰

### 短期计划 (1-2周)
- [ ] 添加Binance价格源
- [ ] 实现价格API接口 (可选，当前为后台服务)
- [ ] 添加价格变化监控
- [ ] 优化批量更新性能

### 中期计划 (1个月)
- [ ] 实现RabbitMQ/Kafka异步队列
- [ ] 添加历史价格存储
- [ ] 实现价格预警功能
- [ ] 支持自定义更新频率

### 长期计划 (3个月)
- [ ] 多价格源聚合和权重
- [ ] 价格异常检测和告警
- [ ] 价格趋势分析
- [ ] 价格API访问控制

## 数据库变更记录

### 最新变更 (价格系统)
```sql
-- 新增支持代币表
CREATE TABLE support_tokens (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(10) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    coingecko_id VARCHAR(50) UNIQUE NOT NULL,
    decimals INTEGER NOT NULL DEFAULT 18,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 添加索引
CREATE INDEX idx_support_tokens_symbol ON support_tokens(symbol);
CREATE INDEX idx_support_tokens_coingecko_id ON support_tokens(coingecko_id);
CREATE INDEX idx_support_tokens_is_active ON support_tokens(is_active);
```

## 部署和运维

### 环境要求
- **开发环境**: Go 1.23+, PostgreSQL 12+, Redis 6+
- **生产环境**: Docker, Kubernetes, Nginx
- **监控**: Prometheus, Grafana
- **日志**: ELK Stack

### 配置管理
- **开发**: config.yaml
- **生产**: 环境变量 + config.prod.yaml
- **密钥**: Vault或环境变量

## 测试策略

### 单元测试
- [ ] 价格服务测试
- [ ] 认证系统测试
- [ ] 数据库操作测试

### 集成测试  
- [ ] API接口测试
- [ ] 数据库集成测试
- [ ] Redis缓存测试

### 性能测试
- [ ] 并发访问测试
- [ ] 价格更新性能测试
- [ ] 数据库性能测试

## 下一步工作重点

1. **完善价格系统** (本周)
   - 添加初始代币数据
   - 测试价格更新功能
   - 完善错误处理

2. **开发资产查询** (下周)  
   - 实现钱包余额查询
   - 集成价格计算
   - 添加资产API接口

3. **智能合约集成** (两周后)
   - 设计时间锁合约
   - 实现合约交互
   - 添加链上事件监听

## 技术决策记录

### 价格系统设计决策
- **选择CoinGecko**: 免费额度够用，API稳定，数据准确
- **使用Redis缓存**: 毫秒级查询响应，支持过期机制
- **30秒更新频率**: 平衡实时性和API限制
- **批量更新**: 减少API调用次数，提高效率
- **异步设计**: 不阻塞主服务，独立的更新循环

这些决策确保了价格系统的高性能、高可用性和良好的扩展性。
