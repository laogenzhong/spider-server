# Spider Vue 管理后台

`admin-console/` 是只在管理员本机启动的 Vue 3 客户端。原有 `admin-vip-cli`、gRPC、App 二进制 RPC 和 App Store 通知接口继续保留，管理后台使用独立的 `/admin-console/*` REST 路由。

## 安全模型

浏览器只访问本机 Vite 服务。`ADMIN_CONSOLE_SECRET` 只由本地 Node/Vite 进程读取，不使用 `VITE_` 前缀，不会进入浏览器 JavaScript 或 `dist` 构建产物。

兑换码管理同样只由本机 Vite 进程处理。CSV、兑换链接和使用状态不会发送到远程 Spider 服务。

本地代理为每个远程请求生成：

- `X-Admin-Timestamp`：秒级 Unix 时间。
- `X-Admin-Nonce`：随机 24 字节 nonce。
- `X-Admin-Signature`：HMAC-SHA256 签名。

签名原文为：

```text
METHOD\nREQUEST_URI\nTIMESTAMP\nNONCE\nSHA256(BODY)
```

服务端使用常量时间比较签名，校验时间窗并拒绝重复 nonce。生产配置默认要求 HTTPS。Vite 服务只绑定 `127.0.0.1`，本地代理也拒绝非 localhost Origin。

## 服务端配置

生成独立密钥：

```bash
openssl rand -hex 32
```

写入服务器实际配置：

```yaml
admin:
  console_secret: "同一份随机密钥"
  console_require_https: true
  console_max_clock_skew: "90s"
  activity_snapshot_at: "23:59:30"
```

也可以使用服务器环境变量，优先级高于 YAML：

```bash
export ADMIN_CONSOLE_SECRET='同一份随机密钥'
```

`console_secret` 留空时会兼容使用原来的 `admin.vip_grant_secret`，但生产环境建议分开配置。

如果服务器位于 Nginx、Caddy 或云负载均衡器之后，TLS 由反向代理终止时需要正确传递：

```text
X-Forwarded-Proto: https
```

## 本地启动

```bash
cd admin-console
cp .env.example .env.local
```

编辑 `.env.local`：

```dotenv
ADMIN_SERVER_URL=https://jjai.top
ADMIN_CONSOLE_SECRET=与服务端相同的随机密钥
```

启动：

```bash
npm install
npm run dev
```

默认地址为 `http://127.0.0.1:4178`。如果端口被占用，Vite 会自动选择下一个可用端口。

## 功能

- 按账号、UID 或 SP 用户 ID 查询用户和当前 Pro 状态。
- 开通 1 分钟、7 天、1 个月、3 个月、1 年或永久 Pro。
- 只撤销后台开通的 Pro，不影响 Apple 购买权益。
- 查询 Apple 交易，并按兑换码或 Apple 购买筛选。
- 查询退款申请 `CONSUMPTION_REQUEST` 和已经生效的退款/撤销。
- 查询今日实时日活 UID，查询历史日活快照 UID。
- 通过独立“今日注册”页面直接查询当天 `users.created_at` 命中的 UID 用户列表。
- 查询指定日期范围内的注册用户。
- 拖拽导入本地 CSV，管理全部兑换码、使用状态、批量序号状态和 LiftTags Pro 回复。
- 读取和修改 iOS 版本更新配置与多语言文案。

## 本地兑换码回复

Admin 的独立本地主库默认保存在：

```text
~/.lifttags-admin/offer-codes.json
```

本地主库不存在时会自动建立空库。把 CSV 拖入页面即可导入：已有兑换码保留原序号和使用状态，只更新变化的兑换链接；新兑换码按顺序追加。页面支持查看、搜索和筛选全部兑换码，也支持用 `1-99` 或 `1-99,105` 批量设置使用状态。

也可以在 `admin-console/.env.local` 修改本地主库路径，或指定首次迁移来源：

```dotenv
LIFTTAGS_OFFER_STORE_PATH=/绝对路径/offer-codes.json
```

这些变量只由本地 Node/Vite 进程读取，不会进入前端构建产物，也不会发送给远程服务。

`offer_type = 3` 识别为兑换码。其他值归类为 Apple 购买，其中历史 `offer_type = 0` 可能包含部署字段前无法识别的旧交易，因此界面显示为“购买 / 历史未知”。

## 日活快照

今日查询直接使用 `users.last_app_enter_at`，UID 本身唯一。每天在 `admin.activity_snapshot_at` 配置的服务器本地时间，将当天命中的 UID 写入 `daily_user_activity_snapshots`，历史查询从快照表读取。

服务启动时会补跑最近应该完成的一天。部署此功能之前的历史日期无法从单一的 `last_app_enter_at` 准确反推，因此不会自动伪造历史数据。
