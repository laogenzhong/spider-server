# Apple IAP 对账、监控与后台接入说明

本文档记录最近新增的两块服务端能力，供后续继续迭代和接入后台系统使用：

- App Store Server API 主动对账
- 支付失败监控表 `apple_payment_failures`

这两块能力的目标是让服务端不只依赖客户端上报和 Apple 通知，还能主动发现漏处理、处理失败、退款/撤销失败、`pending_user` 堆积等问题，并为后台系统提供统一查询入口。

## 相关代码

- `game/appstore/server_api.go`: Go 侧 App Store Server API 适配器。
- `apple_iap_verifier/app_store_api.mjs`: Node 侧调用 Apple 官方 `@apple/app-store-server-library`。
- `game/reconcile/app_store_reconciler.go`: 后台主动对账任务。
- `mysql/model/vip_entitlement_model.go`: 对账结果落库和权益更新。
- `mysql/model/apple_payment_failure_model.go`: 支付失败/告警事件表。
- `gateway/app_store_notification.go`: App Store Server Notifications V2 回调和失败记录。
- `game/router/vip_api.go`: 客户端交易确认验签失败记录。
- `mysql/a_register_mysql.go`: 注册 `ApplePaymentFailure` 自动迁移。
- `config.yaml` / `config.server.example.yaml`: 对账配置项。

## 主动对账

### 配置

主动对账默认关闭。开启前必须配置 App Store Connect 的 App Store Server API Key：

```yaml
app_store:
  environment: "SANDBOX" # TestFlight / Sandbox；正式环境改 PRODUCTION
  app_apple_id: 6776698752
  api_script_path: "apple_iap_verifier/app_store_api.mjs"
  api_key_id: "你的 App Store Server API Key ID"
  api_issuer_id: "你的 Issuer ID"
  api_private_key_path: "/root/app/spiderapi/spider-secrets/AuthKey_xxx.p8"
  reconcile_enabled: true
  reconcile_interval: "6h"
  reconcile_lookback: "720h"
  reconcile_batch_size: 50
  reconcile_max_pages: 10
```

注意：不要直接假设 Sign in with Apple 的 `.p8` 可以复用。这里需要 App Store Server API 可用、并具备 App 内购买相关权限的 API Key。

也支持环境变量覆盖：

- `APP_STORE_API_KEY_ID`
- `APP_STORE_API_ISSUER_ID`
- `APP_STORE_API_PRIVATE_KEY_PATH`
- `APP_STORE_API_PRIVATE_KEY`
- `APP_STORE_RECONCILE_ENABLED`
- `APP_STORE_RECONCILE_INTERVAL`
- `APP_STORE_RECONCILE_LOOKBACK`
- `APP_STORE_RECONCILE_BATCH_SIZE`
- `APP_STORE_RECONCILE_MAX_PAGES`

### 启动行为

服务启动后在 `cmd/main.go` 调用：

```go
reconcile.StartAppStoreReconciler(ctx, cfg.AppStore)
```

如果 `reconcile_enabled: false`，不会启动后台任务。

如果配置不完整，服务不会退出，只会打印：

```text
app store reconcile disabled: server api config is incomplete
```

配置完整后，任务会在启动约 10 秒后执行第一次对账，之后按 `reconcile_interval` 周期执行。

### 对账流程

单轮对账入口：

```go
RunAppStoreReconcileOnce(ctx, cfg)
```

当前执行顺序：

1. 计算时间窗口：`now - reconcile_lookback` 到 `now`。
2. 拉取 App Store Notification History，`onlyFailures=true`，优先补处理 Apple 发送失败或服务端漏掉的通知。
3. 从本地 `apple_transactions` 取一批待对账交易：
   - 按 `updated_at ASC, id ASC` 排序。
   - 按 `uid + original_transaction_id` 去重。
   - 批量大小由 `reconcile_batch_size` 控制。
4. 对每个交易：
   - 拉取 transaction history。
   - 将交易历史写回 `apple_transactions`，并更新 `user_entitlements`。
   - 如果是月订阅商品，拉取 subscription status。
   - 将订阅状态应用到 `user_entitlements`。
   - 按 transaction id 拉取 notification history 并补处理。
5. 检查 `app_store_server_notifications` 中 `pending_user` 总量，写入堆积告警。

### Node API 动作

`apple_iap_verifier/app_store_api.mjs` 支持三类 action：

- `transactionHistory`
- `subscriptionStatus`
- `notificationHistory`

Go 侧统一通过 stdin/stdout 调用 Node 脚本。脚本返回 JSON envelope：

```json
{
  "ok": true,
  "action": "transactionHistory",
  "data": {}
}
```

失败时：

```json
{
  "ok": false,
  "error": "reason",
  "httpStatusCode": 401,
  "apiError": "..."
}
```

Go 侧会把这些失败记录到 `apple_payment_failures`，类别为 `reconcile_failed`。

### 对账写入的数据表

主动对账会影响以下表：

- `apple_transactions`
  - upsert Apple 交易快照。
  - 保存 `transaction_id`、`original_transaction_id`、`product_id`、`expires_at`、`revocation_at`、`signed_transaction_jws` 等。

- `app_store_server_notifications`
  - upsert 拉回来的历史通知。
  - 按 `notification_uuid` 幂等。
  - 处理完成后为 `processed`；无交易或非目标商品为 `ignored`；找不到 uid 为 `pending_user`。

- `user_entitlements`
  - 服务端最终 VIP 权益快照。
  - 永久 VIP 不会被月订阅覆盖。
  - 月订阅按有效期、宽限期、撤销/退款状态更新。

- `apple_payment_failures`
  - 记录主动对账拉取失败、应用失败、退款撤销处理失败、`pending_user` 堆积等。

## 支付失败表

### 表名

```text
apple_payment_failures
```

### 作用

统一保存支付链路中需要人工关注或后台展示的失败事件。写失败表失败时只打印日志，不会影响原支付链路返回。

当前覆盖：

- 客户端交易 JWS 验签失败。
- App Store Server Notification 验签失败。
- 通知入口返回 5xx。
- `pending_user` 无法匹配 uid。
- `pending_user` 堆积。
- `REFUND` / `REVOKE` / `revocationDate` 相关处理失败。
- App Store Server API 主动对账失败。

### 主要字段

- `category`: 失败类别。
- `stage`: 失败阶段。
- `severity`: 告警级别，`warning` 或 `critical`。
- `status`: 处理状态，当前写入默认为 `open`，预留 `resolved`。
- `uid`: 用户 id，匹配不到用户时为 0。
- `order_id`: 服务端预订单 id。
- `product_id`: Apple 商品 id。
- `transaction_id`: Apple 交易 id。
- `original_transaction_id`: Apple 原始交易 id。
- `notification_uuid`: App Store Server Notification UUID。
- `notification_type`: 通知类型，如 `DID_RENEW`、`EXPIRED`、`REFUND`。
- `subtype`: 通知子类型。
- `bundle_id`: Bundle ID。
- `environment`: `SANDBOX` 或 `PRODUCTION`。
- `http_status`: 通知入口返回码，非 HTTP 场景为 0。
- `error_code`: 客户端 API 对应业务错误码，非客户端 API 场景为 0。
- `reason`: 直接错误原因。
- `problem`: 对业务影响的说明，适合后台详情页展示。
- `error_message`: 原始错误字符串。
- `context_json`: 额外排查上下文。
- `occurred_at`: 发生时间。
- `alerted` / `alerted_at`: 告警发送状态预留字段。
- `resolved_at`: 后台标记解决时间预留字段。

### 类别和阶段

| category | stage | severity | 说明 |
| --- | --- | --- | --- |
| `transaction_verify_failed` | `transaction_verify` | `warning` / `critical` | 客户端确认交易时 JWS 验签失败。配置错误为 critical。 |
| `notification_verify_failed` | `notification_verify` | `warning` | Apple 通知 signedPayload 验签失败。 |
| `notification_5xx` | `notification_verify` / `notification_apply` | `critical` | 通知入口返回 503 或 500，Apple 可能重试。 |
| `pending_user` | `pending_user_match` | `warning` | 通知找不到 uid，等待客户端后续确认交易后回放。 |
| `pending_user_backlog` | `pending_user_backlog` | `warning` / `critical` | `pending_user` 堆积。当前数量 >= 20 记为 critical。 |
| `refund_revoke_failed` | `refund_revoke_apply` | `critical` | 退款/撤销通知或交易处理失败，可能导致 VIP 未及时撤销。 |
| `reconcile_failed` | `reconcile` | `warning` | 主动对账拉取或应用失败。 |

### 重点查询 SQL

查看最近未处理失败：

```sql
select *
from apple_payment_failures
where status = 'open'
order by occurred_at desc
limit 50;
```

查看 critical：

```sql
select *
from apple_payment_failures
where status = 'open'
  and severity = 'critical'
order by occurred_at desc;
```

查看退款/撤销失败：

```sql
select *
from apple_payment_failures
where category = 'refund_revoke_failed'
order by occurred_at desc;
```

查看通知 5xx：

```sql
select *
from apple_payment_failures
where category = 'notification_5xx'
order by occurred_at desc;
```

查看 pending_user 堆积：

```sql
select *
from apple_payment_failures
where category = 'pending_user_backlog'
order by occurred_at desc;
```

按用户查看支付失败：

```sql
select *
from apple_payment_failures
where uid = ?
order by occurred_at desc;
```

按原始交易查看支付失败：

```sql
select *
from apple_payment_failures
where original_transaction_id = ?
order by occurred_at desc;
```

## 后台系统接入建议

### 后台 API 命名规范

后台专用功能统一使用 `admin_<domain>_api` 命名，避免和客户端业务接口混在一起。

- Proto 文件：`proto/primary/admin_<domain>_api.proto`，例如 `admin_vip_api.proto`。
- 生成文件：`gen/spider/api/admin_<domain>_api.pb.go` 和 `admin_<domain>_api_grpc.pb.go`。
- Router 文件：`game/router/admin_<domain>_api.go`。
- gRPC service / router 类型：`Admin<Domain>Api`，例如 `AdminVIPApi`。
- gRPC 方法前缀：`/api.Admin<Domain>Api/`，例如 `/api.AdminVIPApi/`。
- 错误码命名：`Admin<Domain>...`，例如 `AdminVIPSecretInvalid`。
- 不要使用业务域名在 `admin` 前面的顺序，也不要把后台 RPC 加进客户端业务 proto。

### 支付失败列表

建议后台先做一个失败列表页，数据源为 `apple_payment_failures`。

建议筛选条件：

- `status`: `open` / `resolved`
- `severity`: `critical` / `warning`
- `category`
- `stage`
- `environment`
- `uid`
- `product_id`
- `transaction_id`
- `original_transaction_id`
- `notification_uuid`
- `occurred_at` 时间范围

建议列表字段：

- `occurred_at`
- `severity`
- `category`
- `stage`
- `status`
- `uid`
- `product_id`
- `transaction_id`
- `original_transaction_id`
- `notification_uuid`
- `reason`
- `problem`

### 支付失败详情

详情页建议展示：

- 基本字段：类别、阶段、级别、状态、发生时间。
- 关联信息：uid、订单、商品、交易、原始交易、通知 uuid。
- 错误信息：`reason`、`problem`、`error_message`。
- 排查上下文：格式化展示 `context_json`。
- 关联表快捷查询：
  - `apple_transactions`
  - `apple_purchase_orders`
  - `app_store_server_notifications`
  - `user_entitlements`

### 告警规则

建议后台或定时任务按以下规则告警：

- `severity = critical and status = open`：立即告警。
- `category = notification_5xx`：立即告警，因为 Apple 会重试，但持续 5xx 会丢通知。
- `category = refund_revoke_failed`：立即告警，因为可能导致已退款用户仍保留 VIP。
- `category = pending_user_backlog` 且最近一条为 `critical`：告警。
- `category = transaction_verify_failed` 且 `problem` 包含配置错误：告警。
- 同一 `category` 在 10 分钟内新增数量超过阈值：告警。

### 处理状态

当前代码只负责写入 `open` 记录，暂未实现后台 API 标记处理完成。

后台接入后建议增加：

- 标记 resolved。
- 记录处理人。
- 记录处理备注。
- 更新 `resolved_at`。
- 可选：更新 `alerted` / `alerted_at`。

目前表里已预留：

- `status`
- `alerted`
- `alerted_at`
- `resolved_at`

如果要记录处理人和备注，需要后续补字段，例如：

- `resolved_by`
- `resolution_note`

### 后续可做的后台操作

这些操作当前还没实现，只是后续后台系统建议：

- 按 `notification_uuid` 重放某条通知。
- 处理所有 `pending_user` 通知。
- 按 `original_transaction_id` 手动触发一次 App Store Server API 对账。
- 按 uid 查询当前 VIP、历史交易和失败事件。
- 对 `refund_revoke_failed` 提供一键重试撤销权益。
- 对 `transaction_verify_failed` 展示配置检查结果，例如根证书、环境、Bundle ID、Node 脚本路径。

## 常见排查路径

### 用户付款了但没有 VIP

1. 查 `apple_payment_failures`，按 uid 或 transaction id。
2. 如果是 `transaction_verify_failed`，检查 JWS、环境、Bundle ID、根证书、Node 脚本。
3. 查 `apple_purchase_orders`，确认订单是否存在、是否过期、商品是否一致。
4. 查 `apple_transactions`，确认交易是否落库。
5. 查 `user_entitlements`，确认权益是否 active。
6. 如已开启主动对账，可按 original transaction id 手动补一次对账。

### Apple 通知一直失败

1. 查 `notification_5xx`。
2. 如果是 503，优先检查 verifier 配置和根证书。
3. 如果是 500，按 `notification_uuid` 查 `app_store_server_notifications`。
4. 如果通知类型是 `REFUND` 或 `REVOKE`，按 critical 处理。

### pending_user 很多

1. 查 `pending_user_backlog` 的 `context_json`。
2. 查最早和最新的 `notification_uuid`。
3. 查对应 `original_transaction_id` 是否已经有客户端确认交易。
4. 如果没有，说明 Apple 通知先到了，但用户本地交易还没补报。
5. 后台后续可提供“按 original transaction id 对账”和“重放 pending_user”按钮。

## 当前限制

- 没有后台 API，只有数据库记录。
- 没有失败记录去重，同一类持续失败会持续写入。
- 没有处理人和处理备注字段。
- 没有自动重放通知或一键修复接口。
- 主动对账默认关闭，必须配置 App Store Server API Key 后启用。

这些限制不影响当前支付链路，但后台接入时应作为下一轮迭代重点。
