# Apple IAP 与 VIP 权益接入记录

本文档记录 Apple 内购接入的当前实现、已知问题和后续待办。后续继续做支付时，应先阅读本文，避免重复讨论已经确认过的风险点。

## 当前目标

App 需要支持两种 VIP 支付：

- 永久 VIP：用户购买一次后，服务端按 `uid` 标记永久 VIP，客户端解锁 VIP。
- 月订阅 VIP：用户按月订阅，服务端按 Apple 交易的过期时间授予 VIP，客户端在有效期内解锁 VIP。

客户端所有 VIP 锁定点只读取本地 `VIPManager.shared.isVIP`。这个状态来自服务端 VIP 状态接口的缓存，不在每个 VIP 功能点实时请求服务器。服务端是最终权威，客户端只在启动、登录后、进入关键页面、购买或恢复购买后、缓存过期时刷新。

## 当前实现

### 服务端

服务端新增 `VIPService`：

```proto
rpc getVIPStatus(GetVIPStatusRequest) returns (VIPStatusResponse);
rpc confirmAppleTransaction(ConfirmAppleTransactionRequest) returns (VIPStatusResponse);
```

主要文件：

- `proto/primary/purchase.proto`
- `game/router/vip_api.go`
- `game/appstore/verifier.go`
- `mysql/model/vip_entitlement_model.go`
- `apple_iap_verifier/verify_transaction.mjs`

当前确认交易流程：

1. 客户端进入 Apple 支付前，先调用 `createApplePurchaseOrder` 创建服务端预订单。
2. 服务端写入 `apple_purchase_orders`，状态为 `created`，默认 30 分钟过期。
3. 客户端拿到 `order_id` 后再发起 StoreKit 支付。
4. 客户端支付成功后提交 `order_id` 与 StoreKit 交易 JWS。
5. Go 服务端调用 Node 子进程验签。
6. Node 使用 Apple 官方 `@apple/app-store-server-library` 验签并解码交易。
7. Go 服务端校验预订单 uid、product id、有效期和 Apple 交易 product id。
8. 服务端按 `transaction_id` upsert `apple_transactions`。
9. 服务端将预订单标记为 `paid`。
10. 服务端写入或更新 `user_entitlements`。
11. 服务端返回最新 VIP 状态。

`apple_transactions.transaction_id` 唯一，客户端重复上报同一笔交易时不会重复授予权益。

注意：预订单只代表“准备支付”的意图，不代表用户已经付款。只创建预订单但没有真实 Apple 支付成功，不会开通 VIP。

### 客户端

客户端购买成功后：

1. 调用服务端 `createApplePurchaseOrder` 创建预订单。
2. 拿到 `order_id` 后进入 StoreKit 支付。
3. StoreKit 2 本地验证交易。
4. 将 `order_id` 与已验证交易写入本地 pending 队列。
5. 调用服务端 `confirmAppleTransaction`。
6. 服务端确认成功后，客户端刷新 `VIPManager` 状态，并从 pending 队列删除交易。
7. 客户端执行 `transaction.finish()`。

如果服务端宕机、网络失败或验签配置暂时不可用，pending 交易不会丢失。客户端会在登录变化、恢复购买、读取 `Transaction.currentEntitlements` 和定时器触发时继续静默补报。

购买页提供“刷新订单状态”按钮。用户手动触发后，客户端会执行 App Store 同步、补报 pending 交易，并刷新服务端 VIP 状态。

## Go 与 Node 交互方式

当前采用子进程方式，不启动独立 HTTP 服务。

Go 执行：

```text
node apple_iap_verifier/verify_transaction.mjs
```

交互协议：

- Go 将 JSON 请求写入 Node 进程 `stdin`。
- Node 将 JSON 结果写入 `stdout`。
- Go 解析 `stdout` 并转换为交易结构。
- Go 使用 context timeout 控制最长执行时间，默认 10 秒。

当前选择子进程的原因：

- 支付确认不是高频接口，先用低复杂度方案打通闭环。
- 不需要额外维护端口、服务发现、健康检查和内网鉴权。
- Node 单次失败只影响当前交易验证请求。

后续如果支付量增大，可以将 `apple_iap_verifier` 改为常驻 Node HTTP 服务。HTTP 服务适合更高并发、更细监控和独立部署，但当前阶段会增加运维复杂度。

## 已知问题与后续待办

### P0: 配置 Apple 根证书

当前 `config.yaml` 中 `app_store.root_certificate_paths` 仍为空。未配置前，`confirmAppleTransaction` 会返回“支付校验服务暂未配置完成”，不会授予权益。

待办：

- 下载 Apple PKI 中用于 App Store Server Library 的 DER `.cer` 根证书。
- 将证书放到服务器安全路径。
- 配置 `app_store.root_certificate_paths` 或环境变量 `APP_STORE_ROOT_CERTIFICATE_PATHS`。
- 用 Sandbox 真实交易验证一次完整购买链路。

### Done: 客户端 pending 交易队列第一版

已完成第一版。当前实现位于 iOS 客户端 `StoreKitManager.swift`：

- 进入 Apple 支付前先创建服务端预订单。
- 购买成功后保存 `order_id` 与 pending 交易，再请求服务端确认。
- 服务端确认成功后删除 pending 交易。
- 服务端确认失败时，交易保留在本地，用户会看到“购买已完成，正在同步 VIP 权益，请稍后刷新”。
- App 启动、登录变化、恢复购买、读取 StoreKit 当前权益和定时器会触发补报。
- 重试有简单退避，避免高频请求。
- pending 记录按 `uid + transactionID` 唯一保存。
- 恢复购买或旧 pending 没有 `order_id` 时，客户端会先补建一个用于对账的预订单，再提交 Apple 交易确认。

已保存字段：

- `orderID`
- `productID`
- `transactionID`
- `originalTransactionID`
- `signedTransactionJWS`
- `createdAt`
- `lastAttemptAt`
- `attemptCount`

后续仍可继续增强：

- 增加网络恢复瞬间触发重试。
- 在购买页展示更完整的 pending 同步状态。
- 区分永久失败与临时失败，例如 product id 错误、验签配置错误、网络失败。

### P0: App Store Server Notifications V2

当前只处理客户端主动上报的交易。订阅续期、退款、撤销、过期、账单重试等事件不能只依赖客户端在线。

待办：

- 新增 App Store Server Notifications V2 HTTP 回调。
- 验证通知 JWS。
- 按 `notificationType` 和 `subtype` 更新 `apple_transactions` 与 `user_entitlements`。
- 覆盖续订、过期、退款、撤销、账单失败、宽限期等状态。
- 保证通知处理幂等。

### P1: 购买页状态防重复点击

如果客户端尚未拿到服务端最新 VIP 状态，购买页可能仍显示购买按钮。Apple 对非消耗型和订阅商品本身会防止真正重复购买，但 UI 体验仍然可能让用户困惑。

待办：

- 进入购买页时强制刷新服务端 VIP 状态。
- 同时读取 StoreKit 当前权益。
- 对已拥有的永久 VIP 隐藏或禁用永久购买按钮。
- 对已订阅的月 VIP 显示当前订阅状态，而不是继续展示普通购买按钮。
- 购买按钮增加提交中状态，防止连续点击。

### P1: 交易确认与 `finish()` 时机

当前购买成功后，客户端会先调用服务端确认，成功后再 `finish()`。这是为了让失败交易能继续被 StoreKit 暴露出来以便补报。

待办：

- 加 pending 队列后，明确失败时是否立即 `finish()`。
- 推荐策略：只要本地交易已经 verified，就先持久化 pending；如果服务端暂时失败，可以在安全保存 pending 后 `finish()`，后续由 pending 队列继续补报。
- 需要实测 StoreKit Sandbox 下未 `finish()` 与已 `finish()` 对 `Transaction.currentEntitlements` 和 `Transaction.updates` 的影响。

### P1: 订阅过期本地刷新

客户端缓存 VIP 状态后，如果月订阅已过期但客户端长时间离线，本地可能短时间仍显示旧状态。

当前已有过期时间判断和周期刷新，但仍需继续打磨。

待办：

- 客户端读取月订阅缓存时，优先按 `expiresAt` 本地判定是否过期。
- 在 `expiresAt` 临近时提高刷新频率。
- 离线时过期必须本地锁回非 VIP，不能等待服务端响应。

### P1: 服务端状态完整性

当前 `user_entitlements` 只保存当前 VIP 权益，`apple_transactions` 保存交易快照。后续接通知后，需要更完整地表达订阅状态。

待办：

- 增加或补充订阅状态字段，例如 grace period、billing retry、revoked、refunded。
- 记录最近一次通知时间和通知原始 JWS。
- 区分 lifetime 与 subscription 的状态更新规则，避免月订阅覆盖永久 VIP。

### P2: Node 子进程性能与监控

当前 Go 每次验证都会启动一次 Node 子进程。支付量低时可以接受，但高并发时开销会变大。

待办：

- 给验签耗时、失败原因、超时次数增加日志或指标。
- 如果支付确认 QPS 上升，将 Node 模块改成常驻 HTTP 服务。
- HTTP 服务需要内网鉴权、健康检查、超时、熔断和部署守护。

### P2: 商品 ID 与环境配置

当前默认商品 ID：

- `hh.spider.vip.monthly`
- `hh.spider.vip.lifetime`

待办：

- 确认 App Store Connect 中商品 ID 与服务端、客户端配置一致。
- Sandbox 使用 `SANDBOX` 环境。
- 上线前切换或确认 Production 环境配置。

## 后续建议顺序

1. 配置 Apple 根证书并跑通 Sandbox 真交易。
2. 客户端增加 pending 交易队列。
3. 购买页按服务端 VIP 状态和 StoreKit 当前权益优化 UI。
4. 服务端接 App Store Server Notifications V2。
5. 完善订阅状态模型和退款/撤销处理。
6. 根据真实调用量决定是否把 Node 子进程升级为常驻 HTTP 服务。
