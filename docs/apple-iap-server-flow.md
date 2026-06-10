# Apple IAP 服务端支付流程

本文档记录 spider-server 当前 Apple 内购服务端流程，包括客户端主动确认、App Store Server Notifications V2、权益落库和后续服务端支付待办。

后台接入和运维查询说明见：`docs/apple-iap-ops-and-admin.md`。

## 目标

- 服务端是 VIP 权益最终权威。
- 客户端只能提交经过 StoreKit 验证的交易 JWS，不能直接开通 VIP。
- App Store 通知用于处理续订、过期、退款、撤销、账单失败和宽限期，不能只依赖客户端在线上报。
- 所有 Apple JWS 都必须验签后再入库和影响权益。
- 通知处理必须幂等，Apple 重试不能导致重复授予或重复撤销。

## 核心文件

- `proto/primary/purchase.proto`: VIPService protobuf 定义。
- `game/router/vip_api.go`: 客户端创建预订单、确认交易、查询 VIP 状态。
- `game/appstore/verifier.go`: Go 到 Node 验签脚本的适配层。
- `game/appstore/server_api.go`: Go 到 App Store Server API Node 脚本的适配层。
- `game/reconcile/app_store_reconciler.go`: 定时主动拉交易历史、订阅状态和通知历史。
- `gateway/app_store_notification.go`: App Store Server Notifications V2 HTTP 回调。
- `mysql/model/vip_entitlement_model.go`: 预订单、交易、通知、当前权益模型和处理规则。
- `apple_iap_verifier/verify_transaction.mjs`: 使用 Apple 官方 Node 库验证交易和通知 JWS。
- `apple_iap_verifier/app_store_api.mjs`: 使用 Apple 官方 Node 库调用 App Store Server API。

## 数据表职责

- `apple_purchase_orders`: 客户端发起购买前创建的服务端预订单。只代表支付意图，不代表已付款。
- `apple_transactions`: Apple 交易快照，按 `transaction_id` 唯一，保存商品、原始交易、过期、退款/撤销、签名时间等信息。
- `app_store_server_notifications`: App Store Server Notifications V2 原始通知和处理状态，按 `notification_uuid` 唯一幂等。
- `apple_payment_failures`: 支付失败和告警事件统一表，记录验签失败、通知 5xx、`pending_user` 堆积、退款/撤销处理失败及主动对账失败。
- `user_entitlements`: 当前用户 VIP 权益快照，客户端 `getVIPStatus` 只读这里。

## 客户端主动确认流程

1. 客户端调用 `createApplePurchaseOrder(product_id)`。
2. 服务端校验商品 ID 是否属于当前配置的 VIP 商品。
3. 服务端写入 `apple_purchase_orders`，状态为 `created`，默认 30 分钟过期，默认 `source=pre_purchase`。
4. 客户端拿到 `order_id` 后调用 StoreKit 发起支付。
5. 支付成功后，客户端把 `order_id`、`product_id`、`transaction_id`、`original_transaction_id`、`signed_transaction_jws` 写入本地 pending 队列。
6. 客户端调用 `confirmAppleTransaction`。
7. 服务端调用 `apple_iap_verifier/verify_transaction.mjs` 验证 `signed_transaction_jws`。
8. Node 使用 `@apple/app-store-server-library` 解出 Apple 交易。
9. 服务端校验预订单 uid、商品 ID、订单状态和交易商品 ID。
10. 服务端按 `transaction_id` upsert `apple_transactions`。
11. 服务端把预订单标记为 `paid`。
12. 服务端按交易当前状态更新 `user_entitlements`。
13. 服务端回放同一 `original_transaction_id` 下此前 `pending_user` 的 App Store 通知。
14. 服务端返回最新 VIP 状态。

### 登录后绑定历史 Apple 购买

未登录购买时，客户端不会调用服务端，也不会存在购买前创建的服务端预订单。用户之后登录并选择恢复/同步购买时，客户端仍先调用 `createApplePurchaseOrder(product_id)` 创建一条用于对账的订单，再提交历史 Apple 交易 JWS。

这类订单在确认时会被标记为 `source=post_login_bind`:

- 不要求 Apple 交易里的 `appAccountToken` 等于当前订单号。
- 使用 Apple 验签后的 `transaction_id` / `original_transaction_id` 做幂等和归属校验。
- 如果同一 `original_transaction_id` 已经绑定其他 uid，则拒绝绑定。
- 月订阅权益按 Apple 交易里的实际 `expiresDate` 计算剩余有效期，已过期交易不会继续授予 VIP。
- 绑定成功后，同一 `original_transaction_id` 的 `pending_user` 通知会被回放。

## App Store Server Notifications V2 流程

Apple 后台通知 URL:

```text
https://<你的域名>/app-store/notifications/v2
```

兼容路径:

```text
https://<你的域名>/app-store-server-notifications/v2
```

处理流程:

1. Apple 向网关发送 JSON: `{ "signedPayload": "..." }`。
2. 网关读取 `signedPayload`，调用 `game/appstore.Verifier.VerifyNotification`。
3. Go 调用 Node 脚本，并传入 bundle id、environment、appAppleId、根证书路径。
4. Node 先验证外层通知 JWS。
5. 如果通知里有 `data.signedTransactionInfo`，Node 继续验证并解码交易 JWS。
6. 如果通知里有 `data.signedRenewalInfo`，Node 继续验证并解码续订 JWS。
7. 服务端用 `notification_uuid` 查询 `app_store_server_notifications`。
8. 已处理过的 `processed` / `ignored` 通知直接返回成功。
9. 服务端通过 `original_transaction_id` 或 `transaction_id` 反查用户:
   - 优先查 `apple_transactions`。
   - 再查已支付 `apple_purchase_orders`。
   - 再查 `user_entitlements`。
10. 找到 uid 后，服务端 upsert `apple_transactions` 并更新 `user_entitlements`。
11. 找不到 uid 时，通知记为 `pending_user`，等待客户端后续主动确认同一原始交易后回放。
12. 处理完成后，通知记为 `processed`；无交易或不支持商品的通知记为 `ignored`。

## 权益更新规则

- 永久 VIP:
  - 有效交易授予永久 VIP。
  - `REFUND`、`REVOKE` 或交易带 `revocationDate` 时撤销对应 `original_transaction_id` 的 VIP。

- 月订阅 VIP:
  - `SUBSCRIBED`、`DID_RENEW`、`RENEWAL_EXTENDED`、`RENEWAL_EXTENSION`、`OFFER_REDEEMED`、`REFUND_REVERSED` 等有效交易会授予或延长 VIP。
  - `REFUND`、`REVOKE` 或交易带 `revocationDate` 时撤销对应订阅。
  - `EXPIRED`、`GRACE_PERIOD_EXPIRED` 会关闭对应订阅。
  - `DID_FAIL_TO_RENEW` 如果交易还没过期，或仍处于宽限期，则保留到有效期或宽限期结束；否则关闭。
  - `DID_CHANGE_RENEWAL_STATUS`、`DID_CHANGE_RENEWAL_PREF`、`PRICE_INCREASE`、`REFUND_DECLINED`、`TEST` 只记录状态，不主动改变当前权益。

## 幂等和乱序处理

- 客户端确认交易按 `apple_transactions.transaction_id` 幂等。
- Apple 通知按 `app_store_server_notifications.notification_uuid` 幂等。
- 通知先于客户端确认到达时，不丢弃，先标记为 `pending_user`。
- 客户端后续确认交易成功后，会回放同一 `original_transaction_id` 的 `pending_user` 通知。
- 永久 VIP 不会被月订阅覆盖。用户已有永久 VIP 时，月订阅通知不会降级当前权益。

## 部署检查

上线或联调前确认:

- `app_store.bundle_id` 与 App 的 Bundle ID 一致。
- Sandbox 配置 `app_store.environment: "SANDBOX"`。
- Production 配置 `app_store.environment: "PRODUCTION"`，并设置真实 `app_store.app_apple_id`。
- `app_store.root_certificate_paths` 指向 Apple 根证书 DER `.cer` 文件。
- 主动对账需要配置 `app_store.api_key_id`、`app_store.api_issuer_id`、`app_store.api_private_key_path`，并打开 `app_store.reconcile_enabled`。
- 服务器外网域名支持 HTTPS，并能被 Apple 访问。
- App Store Connect 的 App Store Server Notifications V2 URL 指向 `/app-store/notifications/v2`。
- 服务端日志能看到 `app store notifications endpoint` 启动输出。
- `apple_iap_verifier` 已执行 `npm install`，服务器上 `node` 可执行。

## 后续服务端支付待办

### P0: Sandbox 全链路实测

- 用 Sandbox 账号完成永久 VIP 和月订阅真实购买。
- 验证客户端主动确认能写入 `apple_transactions` 和 `user_entitlements`。
- 在 App Store Connect 发送 Server Notifications V2 测试通知，确认回调能验签和入库。
- 实测续订、过期、退款/撤销、账单失败或宽限期能正确改变权益。

### Done: App Store Server API 主动对账

服务端已接入主动拉 Apple 状态的能力。打开 `app_store.reconcile_enabled` 后，后台任务会按 `app_store.reconcile_interval` 定时执行:

- 查询通知历史，并优先补处理 Apple 发送失败的通知。
- 基于本地 `apple_transactions` 里的历史交易批量查询交易历史。
- 对月订阅查询订阅当前状态。
- 将拉回来的交易、订阅状态和通知历史回写到 `apple_transactions`、`app_store_server_notifications`、`user_entitlements`。

主动对账需要 App Store Connect 里的 App Store Server API Key:

- `app_store.api_key_id`
- `app_store.api_issuer_id`
- `app_store.api_private_key_path`

不要直接假设 Sign in with Apple 的 `.p8` 可以复用；需要使用具备 App 内购买相关权限的 App Store Server API 密钥。

### P0: 生产环境配置与密钥管理

- 把 Sandbox 和 Production 配置拆清楚。
- 生产环境设置 `app_apple_id: 6776698752`。
- `.p8`、Apple 根证书、数据库密码只走服务器安全路径或环境变量。
- 确认 `.gitignore` 覆盖 `*.p8` 和 `config.local.yaml`。

### Done: 支付状态观测和告警基础表

已新增 `apple_payment_failures` 表作为支付失败和告警事件统一入口，当前会记录:

- 客户端交易 JWS 验签失败，包括 uid、订单、商品、transaction id、original transaction id、环境、JWS 长度和错误原因。
- App Store Server Notification 验签失败，包括 signedPayload 长度、点号数量和错误原因。
- 通知入口返回 5xx，包括验签配置错误导致的 503、通知处理失败导致的 500。
- `pending_user` 通知无法匹配 uid，以及主动对账时发现的 `pending_user` 堆积数量。
- `REFUND`、`REVOKE` 或带 `revocationDate` 的交易处理失败，按 `critical` 记录。
- App Store Server API 主动对账拉取或应用失败。

表内重点字段:

- `category`: 失败类别，例如 `transaction_verify_failed`、`notification_5xx`、`pending_user_backlog`、`refund_revoke_failed`。
- `stage`: 失败阶段，例如 `transaction_verify`、`notification_apply`、`refund_revoke_apply`。
- `severity`: 告警级别，`warning` 或 `critical`。
- `reason`: 直接错误原因。
- `problem`: 对业务影响的解释。
- `context_json`: 额外排查上下文。

后续仍建议增加运维查询接口或命令，方便按 uid、transaction id、original transaction id 查支付状态。

### P1: 订阅状态模型增强

当前 `user_entitlements` 只存当前 VIP 快照。后续建议补更完整的订阅状态:

- 是否处于 billing retry。
- 宽限期结束时间。
- 自动续订状态。
- 过期原因。
- 最近一次通知类型和时间。
- 是否退款、撤销、家庭共享撤销。

这样客户端可以展示更准确的订阅状态，而不只是 VIP/非 VIP。

### P1: 通知回放与修复工具

- 做一个后台命令或管理接口，用来重放某条 `notification_uuid`。
- 做一个命令处理所有 `pending_user` 通知。
- 支持按 `original_transaction_id` 重新拉 Apple 状态后修复权益。

### P2: Node 验签服务化

当前每次验签都会启动 Node 子进程，支付量低时可以接受。后续如果支付请求或通知量上来:

- 把 `apple_iap_verifier` 改成常驻 HTTP 服务。
- 加健康检查、超时、重试、熔断。
- 给内部 HTTP 服务加鉴权，只允许后端内网调用。

### P2: 自动化测试

- 给 `vip_entitlement_model.go` 增加通知状态机单元测试。
- 覆盖续订、过期、退款、撤销、宽限期、通知乱序、重复通知、pending_user 回放。
- 给网关通知 handler 加无效 JSON、空 payload、验签失败、处理成功的测试。
