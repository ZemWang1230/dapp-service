# 邮箱模块使用指南

## 概述

邮箱模块实现了基于"方案B"的邮箱订阅和通知系统，支持用户添加多个邮箱、验证邮箱、订阅合约通知等功能。

## 功能特性

### 1. 邮箱管理
- ✅ 添加邮箱地址（支持备注）
- ✅ 查看邮箱列表
- ✅ 修改邮箱备注
- ✅ 删除邮箱

### 2. 邮箱验证
- ✅ 发送验证码（6位数字，有效期可配置）
- ✅ 验证码验证（支持过期检查）
- ✅ 自动清理过期验证码（定时任务）

### 3. 订阅管理
- ✅ 创建合约通知订阅
- ✅ 查看订阅列表
- ✅ 更新订阅配置
- ✅ 删除订阅

### 4. 邮件通知
- ✅ SMTP邮件发送
- ✅ HTML格式邮件模板
- ✅ 流程状态变更通知
- ✅ 发送日志记录和去重

## API 接口

### 邮箱管理

#### 添加邮箱
```http
POST /api/v1/emails
Authorization: Bearer <token>
Content-Type: application/json

{
  "email": "user@example.com",
  "remark": "工作邮箱"
}
```

#### 获取邮箱列表
```http
GET /api/v1/emails?page=1&page_size=10
Authorization: Bearer <token>
```

#### 更新邮箱备注
```http
PUT /api/v1/emails/{id}/remark
Authorization: Bearer <token>
Content-Type: application/json

{
  "remark": "新的备注"
}
```

#### 删除邮箱
```http
DELETE /api/v1/emails/{id}
Authorization: Bearer <token>
```

### 邮箱验证

#### 发送验证码
```http
POST /api/v1/emails/send-verification
Authorization: Bearer <token>
Content-Type: application/json

{
  "user_email_id": 123
}
```

#### 验证邮箱
```http
POST /api/v1/emails/verify
Authorization: Bearer <token>
Content-Type: application/json

{
  "user_email_id": 123,
  "code": "123456"
}
```

### 订阅管理

#### 创建订阅
```http
POST /api/v1/emails/subscriptions
Authorization: Bearer <token>
Content-Type: application/json

{
  "user_email_id": 123,
  "timelock_standard": "compound",
  "chain_id": 1,
  "contract_address": "0x1234567890abcdef1234567890abcdef12345678",
  "notify_on": ["waiting", "ready", "executed"]
}
```

#### 获取订阅列表
```http
GET /api/v1/emails/subscriptions?page=1&page_size=10
Authorization: Bearer <token>
```

#### 更新订阅
```http
PUT /api/v1/emails/subscriptions/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "notify_on": ["ready", "executed", "cancelled"],
  "is_active": true
}
```

#### 删除订阅
```http
DELETE /api/v1/emails/subscriptions/{id}
Authorization: Bearer <token>
```

## 配置说明

在 `config.yaml` 中配置邮件服务：

```yaml
email:
  smtp_host: "smtp.zoho.com"
  smtp_port: 587
  smtp_username: "zem@timelock.live"
  smtp_password: "X1p1DdL7aHBd"
  from_name: "TimeLocker Notification"
  from_email: "zem@timelock.live"
  verification_code_expiry: "10m"        # 验证码过期时间
  email_url: "http://16.163.43.186:8080" # 前端基础URL
```

## 数据库表结构

### 1. emails（邮箱主表）
- `id`: 主键
- `email`: 邮箱地址（唯一）
- `is_deliverable`: 是否可投递
- `created_at`, `updated_at`: 时间戳

### 2. user_emails（用户邮箱关系表）
- `id`: 主键
- `user_id`: 用户ID（外键）
- `email_id`: 邮箱ID（外键）
- `remark`: 备注
- `is_verified`: 是否已验证
- `last_verified_at`: 最后验证时间
- `created_at`, `updated_at`: 时间戳

### 3. email_verification_codes（验证码表）
- `id`: 主键
- `user_email_id`: 用户邮箱关系ID（外键）
- `code`: 验证码
- `expires_at`: 过期时间
- `sent_at`: 发送时间
- `attempt_count`: 尝试次数
- `is_used`: 是否已使用

### 4. user_email_subscriptions（订阅表）
- `id`: 主键
- `user_email_id`: 用户邮箱关系ID（外键）
- `timelock_standard`: Timelock标准（compound/openzeppelin）
- `chain_id`: 链ID
- `contract_address`: 合约地址
- `notify_on`: 通知状态（JSON数组）
- `is_active`: 是否激活
- `created_at`, `updated_at`: 时间戳

### 5. email_send_logs（发送日志表）
- `id`: 主键
- `email_id`: 邮箱ID（外键）
- `flow_id`: 流程ID
- `timelock_standard`: Timelock标准
- `chain_id`: 链ID
- `contract_address`: 合约地址
- `status_from`, `status_to`: 状态变更
- `tx_hash`: 交易哈希
- `send_status`: 发送状态（success/failed）
- `error_message`: 错误信息
- `retry_count`: 重试次数
- `sent_at`: 发送时间

## 通知状态

支持的通知状态：
- `waiting`: 等待中
- `ready`: 就绪
- `executed`: 已执行
- `cancelled`: 已取消
- `expired`: 已过期（仅Compound）

## 定时任务

系统会自动运行以下定时任务：
- **验证码清理**: 每30分钟清理一次过期的验证码
- **流程状态刷新**: 根据配置定期检查和更新流程状态

## 安全特性

1. **验证码限制**: 防止频繁发送（1分钟间隔）
2. **邮箱验证**: 只有验证过的邮箱才能接收通知
3. **用户隔离**: 每个用户的邮箱和订阅完全独立
4. **发送去重**: 同一流程状态变更不会向同一邮箱重复发送

## 错误处理

- **404**: 资源不存在（邮箱、订阅等）
- **409**: 资源冲突（邮箱已存在、订阅已存在）
- **422**: 验证码无效或已过期
- **429**: 请求过于频繁（验证码发送限制）
- **400**: 请求参数错误
- **500**: 服务器内部错误

## 使用示例

### 完整流程示例

1. **添加邮箱**
```bash
curl -X POST http://localhost:8080/api/v1/emails \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "remark": "主邮箱"}'
```

2. **发送验证码**
```bash
curl -X POST http://localhost:8080/api/v1/emails/send-verification \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user_email_id": 1}'
```

3. **验证邮箱**
```bash
curl -X POST http://localhost:8080/api/v1/emails/verify \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user_email_id": 1, "code": "123456"}'
```

4. **创建订阅**
```bash
curl -X POST http://localhost:8080/api/v1/emails/subscriptions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_email_id": 1,
    "timelock_standard": "compound",
    "chain_id": 1,
    "contract_address": "0x1234567890abcdef1234567890abcdef12345678",
    "notify_on": ["waiting", "ready", "executed"]
  }'
```

## 注意事项

1. 所有API都需要用户认证（Bearer Token）
2. 邮箱必须先验证才能用于订阅通知
3. 验证码有效期为10分钟（可配置）
4. 同一用户不能重复添加相同的邮箱地址
5. 同一邮箱不能重复订阅相同的合约
