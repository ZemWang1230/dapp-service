## Timelock 交易流关联与邮件通知设计（采用邮箱订阅方案B）

### 目标

- 仅以链上事件为起点：从 QueueTransaction（Compound）/ CallScheduled（OpenZeppelin）开始生成 `flow`，不依赖任何“前提条件”。
- 贯通两张事件表（`compound_timelock_transactions`、`openzeppelin_timelock_transactions`）与一张流程表（`timelock_transaction_flows`），确保全流程状态及时、幂等更新。
- 对“用户发起”的 Timelock 流程，采用《email_subscription_design.md》中“方案B”的邮箱订阅模型进行通知。
- 在无事件阶段（waiting → ready、以及 Compound 的过期）通过定时任务按 ETA 推进，保证用户及时获知。

---

### 不依赖前提条件与起点规则

- flow 的创建仅源自“排队/调度事件”：
  - Compound：QueueTransaction → 创建 `flow`（status=waiting）。
  - OpenZeppelin：CallScheduled → 创建 `flow`（status=waiting）。
- 不从 propose 或治理层面事件创建 flow；也不依赖任何外部前置数据。扫描到上述事件即开始。

---

### 流程状态模型与事件映射

- 统一状态：`waiting`、`ready`、`executed`、`cancelled`、`expired`（仅 Compound 存在 `expired`）。
- 事件到状态：
  - Compound：
    - QueueTransaction → waiting；`eta` = 事件 `event_eta`；`expired_at` = `eta + grace_period`（来自 `compound_timelocks.grace_period`）。
    - ExecuteTransaction → executed（记录 `execute_tx_hash`、`executed_at = block_timestamp`）。
    - CancelTransaction → cancelled（记录 `cancel_tx_hash`、`cancelled_at = block_timestamp`）。
  - OpenZeppelin：
    - CallScheduled → waiting；`eta` = `block_timestamp + event_delay`。
    - CallExecuted → executed。
    - Cancelled → cancelled。

FlowID 规则：
- Compound：`flow_id = bytes32 txHash`（事件索引参数，对应模型中的 `event_tx_hash`）。
- OpenZeppelin：`flow_id = bytes32 id`（事件索引参数，对应模型中的 `event_id`）。

唯一键：
- `(flow_id, timelock_standard, chain_id, contract_address)`；重复事件通过幂等 upsert 保护。

---

### 与两张事件表协同

- 所有链上事件原始数据先落两张事件表：
  - `compound_timelock_transactions`、`openzeppelin_timelock_transactions`（当前仓储已用 ON CONFLICT DO NOTHING 去重）。
- `EventProcessor.processCompoundFlow/processOpenZeppelinFlow` 在事件入库同时：
  - 以“通用匹配键”加载/创建 `timelock_transaction_flows`；
  - 根据事件类型写入对应字段（queue/execute/cancel tx hash、时间戳、eta/expired_at 等），并推进状态。
- 这样，事件表是事实来源，流程表是聚合视图，任何时刻两者一致可回放验证。

---

### 无事件状态推进（等待→就绪、以及过期）

任务：FlowStatusRefresher（建议 30s~60s 运行一次，可配置）
- waiting → ready：扫描 `status=waiting AND eta <= now()` 的 flow 批量更新为 `ready`。
- expired（仅 Compound）：扫描 `status IN (waiting, ready) AND expired_at <= now()` 且尚未 `executed/cancelled` 的 flow 更新为 `expired`。
- 每次推进需要幂等保护（判断当前 `status` 是否仍允许前进）。

建议仓储接口：
- `GetWaitingFlowsDue(ctx, now, limit)`、`GetCompoundFlowsExpired(ctx, now, limit)`、`UpdateFlowStatus(ctx, flowID, fromStatus, toStatus)`。

---

### “方案B”邮箱订阅模型对接

沿用《email_subscription_design.md》的“方案B”表结构：
- `emails (id, email, ...)`：邮箱主表（去重主体）。
- `user_emails (id, user_id, email_id, is_verified, remark, ...)`：用户与邮箱的关系与验证。
- `user_email_subscriptions (id, user_email_id, timelock_standard, chain_id, contract_address, notify_on, is_active, ...)`：订阅按“用户-邮箱关系”隔离。
- `email_send_logs (id, email_id, flow_id, timelock_standard, chain_id, contract_address, status_from, status_to, tx_hash, send_status, error_message, retry_count, sent_at, UNIQUE(email_id, flow_id, status_to))`：按邮箱维度去重。

通知对象识别（“发起人”）：
- 在创建 flow 时记录 `initiator_address = from_address`（为 `timelock_transaction_flows` 增加该字段）。
- 仅向符合以下条件的邮箱发送：
  - 存在本地用户 `users.wallet_address = initiator_address`；
  - 该用户下所有 `user_emails.is_verified = true` 的邮箱；
  - 该邮箱对应的 `user_email_subscriptions` 至少一条匹配 `(standard, chain_id, contract_address)`，且 `notify_on` 包含本次 `status_to`，并且 `is_active = true`。

最终邮件去重：
- 汇总所有命中的订阅后，转换为邮箱集合 `email_id`，再以 `email_send_logs (email_id, flow_id, status_to)` 做幂等过滤，避免多用户订阅导致对同一邮箱重复发送。

---

### 为什么建议新增 `initiator_address`

核心动机：
- 无事件推进场景必须有“发起人”上下文。`waiting→ready`、`expired` 由定时任务推进，任务本身没有当前链上事件，无法即时得到 `from_address`；将 `initiator_address` 固化到 `flow`，定时任务可直接用来匹配订阅并通知。
- 降低耦合与成本。避免每次通知都回查事件表、寻找“最早 queue/schedule 交易”的 `from_address`（跨表/排序/聚合，性能差、实现复杂、乱序下易出错）。
- 语义稳定、幂等友好。`flow` 作为聚合视图仅在创建时写一次“发起人”，后续状态前进不影响通知对象，有利于 `email_send_logs` 幂等与审计。
- 跨标准统一。Compound 与 OpenZeppelin 的“发起人”统一定义为 queue/schedule 交易的 `from_address`，避免各标准分别回溯。
- 审计与可观测性。`flow` 直接展示“谁发起、何时发起”，便于排障、可视化与导出。

备选方案的不足：
- 动态回溯事件表获取 `from_address` 在无事件推进路径上尤其脆弱（可能需要选最早一条 queue/schedule、处理重复/重扫、事件乱序、额外排序与分页）。

实现与索引建议：
- 迁移为 `timelock_transaction_flows` 新增 `initiator_address VARCHAR(42)`，在创建 flow（Queue/CallScheduled）时写入。
- 按需新增索引 `(initiator_address)` 支持发起人维度检索或统计。

---

### 触发点与伪 SQL/逻辑

触发时机：
- 事件入库并完成 `flow` 状态推进后（EventProcessor 内）。
- FlowStatusRefresher 将 flow 从 waiting→ready 或置为 expired 后。

查找收件邮箱（对邮箱去重）：

```sql
-- 已知: $standard, $chain_id, $contract_address, $flow_id, $status_to
WITH subs AS (
  SELECT s.id AS subscription_id,
         ue.id AS user_email_id,
         e.id  AS email_id
  FROM user_email_subscriptions s
  JOIN user_emails ue ON ue.id = s.user_email_id
  JOIN emails e       ON e.id  = ue.email_id
  JOIN users u        ON u.id  = ue.user_id
  WHERE s.is_active = true
    AND ue.is_verified = true
    AND s.timelock_standard = $standard
    AND s.chain_id = $chain_id
    AND s.contract_address = $contract_address
    AND s.notify_on @> to_jsonb(array[$status_to]::text[])
    AND lower(u.wallet_address) = lower($initiator_address)
)
SELECT DISTINCT email_id FROM subs;
```

发送与日志：
- 对每个 `email_id`：若不存在 `email_send_logs (email_id, flow_id, status_to)` 记录，则发送并写入日志；失败记录错误并支持重试（指数退避）。

---

### 幂等、乱序与一致性

- 事件乱序：根据 `block_number` 与事件类型的时序，仅允许状态单向前进（waiting → ready → executed/cancelled/expired）。
- 重复事件：事件表 `ON CONFLICT DO NOTHING`，流程更新前校验状态避免重复写。
- 通知幂等：`email_send_logs` 的唯一键保障一次变更对同一邮箱仅发一封。

---

### 配置与运行

- 邮件：使用 `config.yaml#email` 的 SMTP 配置；可扩展第三方发送商。
- 刷新：新增 `scanner.flow_refresh_interval`、`scanner.flow_refresh_batch_size`（默认 30s / 200）。
- 确认数：如需稳妥，可在 ready 推进前要求“当前扫描高度 ≥ 事件区块 + confirmations”。

---

### 迁移与开发任务

数据库迁移：
- 新增“方案B”四张表：`emails`、`user_emails`、`user_email_subscriptions`、`email_send_logs`。
- 建议为 `timelock_transaction_flows` 增加 `initiator_address VARCHAR(42)` 字段。

服务与仓储：
- 完成 `EventProcessor.processCompoundFlow/processOpenZeppelinFlow` 的 flow upsert 与状态推进。
- 新增 `FlowStatusRefresher` 定时任务（waiting→ready、Compound 的 expired）。
- 新增 EmailService：`SendFlowNotification(ctx, toEmail, walletAddress, flow, transition, txHash)`；落地发送日志与重试。
- 统一代码中的状态枚举为 `waiting/ready/executed/cancelled/expired`（当前仓储中存在 `proposed/queued` 的历史常量）。

验收口径：
- 从导入 timelock 合约开始，实际在链上执行 queue/schedule → ETA 到达 → execute/cancel：
  - 事件表与流程表一致；
  - ETA 到达后 ≤ 刷新周期推进到 `ready`，即使期间无新事件；
  - 发起人用户已验证的订阅邮箱收到各状态变更邮件，且同一邮箱不重复。

