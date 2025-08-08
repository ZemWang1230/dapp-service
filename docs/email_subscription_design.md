## 邮箱订阅与通知数据库设计（含可优化方案）

### 目标与场景

- 用户可添加多个邮箱（需要验证；未验证可显示但不可用；可重发验证码；添加者可修改备注）。
- 用户为邮箱选择需要监听的合约（按标准、链、合约地址），当 `timelock_transaction_flows` 某流程触发通知条件时，向被选择的邮箱发送通知。
- 一个邮箱可被多个不同用户添加（且每个用户可对该邮箱写不同备注）。
- 不同用户为同一邮箱订阅的合约集合可能不同；同一流程变更同一邮箱仅应发送一次（去重）。

---

## 方案对比

### 方案 A（邮箱为订阅主体，订阅共享，推荐用于“最小数据冗余”）

- 订阅表以邮箱为主体（`email_id`），同一邮箱对同一合约只有一条订阅，多个用户通过“所有者”表共享该订阅。
- 优点：
  - 去重天然简单（发送目标是邮箱）。
  - 同一邮箱对同一合约只有一条订阅记录，数据最少。
- 缺点：
  - 需通过“所有者”隔离可见性；逻辑上不同用户间存在隐式耦合（使用同一邮箱时）。

### 方案 B（订阅属于用户-邮箱关系，强隔离，推荐用于“权限与隐私清晰”的优化方案）

- 订阅表以用户-邮箱关系（`user_email_id`）为主体，每个用户对同一邮箱可有完全独立的订阅集合。
- 优点：
  - 用户隔离彻底：互不影响、互不可见；“备注”“验证状态”等都与用户-邮箱关系绑定。
  - 满足“未验证不可使用”的用户维度需求（即同一邮箱被多用户添加，A 已验证不影响 B 的未验证状态）。
- 缺点：
  - 相比方案 A，订阅总量可能更多；发送端仍需对“邮箱”做一次去重（避免同一邮箱因多位用户的订阅导致重复发送）。

> 综合业务诉求（每个用户对同一邮箱的备注/验证/使用独立，且不同人可能订阅不同合约），“方案 B”为更清晰的优化方案。下面重点给出方案 B 的设计细节，并附带方案 A 的表结构以备选。

---

## 方案 B：数据库设计（推荐）

### 1) 邮箱与用户关系

- `emails`（邮箱主表，邮箱唯一）
  - id BIGSERIAL PK
  - email VARCHAR(200) UNIQUE NOT NULL
  - is_deliverable BOOLEAN NOT NULL DEFAULT true  // 可选：用于第三方验证结果缓存
  - created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
  - 索引：UNIQUE(email)

- `user_emails`（用户与邮箱的关系与属性）
  - id BIGSERIAL PK
  - user_id BIGINT NOT NULL REFERENCES users(id)
  - email_id BIGINT NOT NULL REFERENCES emails(id)
  - remark VARCHAR(200) NULL  // 添加者可修改的备注
  - is_verified BOOLEAN NOT NULL DEFAULT false  // 此用户对该邮箱的验证状态
  - last_verified_at TIMESTAMPTZ NULL
  - created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
  - 约束：UNIQUE(user_id, email_id)
  - 索引：(user_id)、(email_id)

- `email_verification_codes`（验证码与限流）
  - id BIGSERIAL PK
  - user_email_id BIGINT NOT NULL REFERENCES user_emails(id) ON DELETE CASCADE
  - code VARCHAR(16) NOT NULL
  - expires_at TIMESTAMPTZ NOT NULL
  - sent_at TIMESTAMPTZ NOT NULL DEFAULT now()
  - attempt_count INTEGER NOT NULL DEFAULT 0
  - is_used BOOLEAN NOT NULL DEFAULT false
  - 复合索引：(user_email_id, is_used)

说明：
- “未验证可显示但不可用”：下游选择发送时只允许 `user_emails.is_verified = true` 的记录参与订阅匹配。
- “可重发验证码”：在发送新验证码前检查 `email_verification_codes` 的 `sent_at` 与配置的最小重发间隔。

### 2) 订阅（按用户-邮箱隔离）

- `user_email_subscriptions`
  - id BIGSERIAL PK
  - user_email_id BIGINT NOT NULL REFERENCES user_emails(id) ON DELETE CASCADE
  - timelock_standard VARCHAR(20) NOT NULL CHECK (timelock_standard IN ('compound','openzeppelin'))
  - chain_id INTEGER NOT NULL
  - contract_address VARCHAR(42) NOT NULL
  - notify_on JSONB NOT NULL DEFAULT '[]'  // e.g. ["waiting","ready","executed","cancelled","expired"]
  - is_active BOOLEAN NOT NULL DEFAULT true
  - created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
  - 约束：UNIQUE(user_email_id, timelock_standard, chain_id, contract_address)
  - 索引：(timelock_standard, chain_id, contract_address)、(user_email_id)

说明：
- 每个用户对同一邮箱的订阅集合互相独立，满足“不同的人需要监听的合约不一样”。
- `notify_on` 可进一步扩展为包含函数签名/目标地址等过滤（新增 `notify_filters` JSONB）。

### 3) 发送日志（按邮箱去重）

- `email_send_logs`
  - id BIGSERIAL PK
  - email_id BIGINT NOT NULL REFERENCES emails(id)
  - flow_id VARCHAR(128) NOT NULL
  - timelock_standard VARCHAR(20) NOT NULL
  - chain_id INTEGER NOT NULL
  - contract_address VARCHAR(42) NOT NULL
  - status_from VARCHAR(20) NULL
  - status_to VARCHAR(20) NOT NULL
  - tx_hash VARCHAR(66) NULL
  - send_status VARCHAR(20) NOT NULL CHECK (send_status IN ('success','failed'))
  - error_message TEXT NULL
  - retry_count INTEGER NOT NULL DEFAULT 0
  - sent_at TIMESTAMPTZ NOT NULL DEFAULT now()
  - 去重约束：UNIQUE(email_id, flow_id, status_to)
  - 索引：(flow_id, status_to)、(timelock_standard, chain_id, contract_address)

说明：
- 由于订阅按 `user_email_id` 分裂，同一邮箱可能被多个用户订阅到同一流程；最终发送前以 `email_id + flow_id + status_to` 去重，保证一封即可。

### 4) 通知匹配与发送（关键 SQL/逻辑）

伪 SQL：从流程变更找到收件邮箱（多用户、多订阅合并后对邮箱去重）

```sql
-- 基于流程变更（flow: standard, chain_id, contract_address, flow_id, status_to）
WITH subs AS (
  SELECT s.id AS subscription_id,
         ue.id AS user_email_id,
         e.id  AS email_id
  FROM user_email_subscriptions s
  JOIN user_emails ue ON ue.id = s.user_email_id
  JOIN emails e       ON e.id  = ue.email_id
  WHERE s.is_active = true
    AND ue.is_verified = true
    AND s.timelock_standard = $1
    AND s.chain_id = $2
    AND s.contract_address = $3
    AND s.notify_on @> to_jsonb(array[$4]::text[])
)
SELECT DISTINCT email_id FROM subs;
```

之后：
- 对每个 `email_id` 执行去重检查：`NOT EXISTS (SELECT 1 FROM email_send_logs WHERE email_id=$email_id AND flow_id=$flow_id AND status_to=$status_to)`。
- 通过 SMTP/第三方服务发送邮件，写入 `email_send_logs`，失败可 `retry_count+1` 及退避重试。

### 5) 验证流程与限流

- 发送验证码：
  - 依据 `user_email_id` 生成验证码，写 `email_verification_codes`；受最小重发间隔、最大重发次数限制。
- 验证：
  - 校验 `code` 与 `expires_at`，匹配 `user_email_id`，设置 `user_emails.is_verified=true, last_verified_at=now()`。
  - `emails.is_deliverable` 可作为可选增强（例如使用外部验证结果缓存），非必需。

### 6) 索引与性能

- 核心查询索引：`user_email_subscriptions (timelock_standard, chain_id, contract_address)`。
- 去重查询索引：`email_send_logs (email_id, flow_id, status_to)` UNIQUE。
- 关系索引：`user_emails (user_id)`、`(email_id)`；`emails (email)` UNIQUE。

### 7) 并发与幂等

- 发送幂等：依赖 `email_send_logs` 的 UNIQUE(email_id, flow_id, status_to)。
- 订阅幂等：依赖 `user_email_subscriptions` 的 UNIQUE(user_email_id, standard, chain_id, contract_address)。
- 验证幂等：`email_verification_codes.is_used` 与过期校验。

### 8) UI/权限建议

- 用户的“我的邮箱”：来自 `user_emails`，每条带 `remark`、`is_verified`、重发验证码、删除/编辑。
- 用户的“我的订阅”：来自 `user_email_subscriptions`（仅显示自己的 `user_email_id` 下的订阅）。
- 发送时遵循“该用户是否验证过该邮箱”的限制（`ue.is_verified=true`）。

---

## 方案 A：数据库设计（邮箱为订阅主体，备选）

若更关注最小化订阅行数，可改为：

- `emails`（同上）
- `user_emails`（同上，含用户侧验证与备注）
- `email_timelock_subscriptions`（以 email_id 为主体）
  - id PK, email_id, standard, chain_id, contract_address, notify_on, is_active
  - UNIQUE(email_id, standard, chain_id, contract_address)
- `subscription_owners`（订阅所有者，用于权限与可见性）
  - subscription_id, user_id UNIQUE(subscription_id, user_id)
- `email_send_logs`（同上・按 email_id 去重）

差异点：
- 同一邮箱、同一合约仅一条订阅，减少重复数据。
- 但需要约束可见性：仅向订阅 owners 展示管理权限；当 owner 列表为空时可自动 `is_active=false`。
- 用户验证仍依据 `user_emails.is_verified`；发送时只要邮箱存在至少一个 owner 验证通过即可（或更严格：仅当“产生该订阅的用户集合中至少一人已验证”才触发，需业务定义）。

---

## 结论与推荐

- 若优先“权限/隐私清晰、与用户动作完全对齐”，推荐采用“方案 B（订阅属于用户-邮箱关系）”。
  - 每个用户的订阅、验证、备注互相独立；
  - 发送端最终仍按 email 去重，避免向同一邮箱重复发送。

- 若优先“尽量减少订阅重复记录”，可选“方案 A（邮箱为订阅主体 + owners）”，但需在权限展示与管理上额外处理共享邮箱的可见性问题。


