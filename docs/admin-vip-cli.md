# Admin VIP CLI

`cmd/admin_vip_cli` 是后台开通 VIP 的独立命令行工具。

它会：

- 读取 `config.yaml` 或 `SPIDER_SERVER_CONFIG` 指定的配置。
- 连接 MySQL 查询账号、昵称和当前 VIP 状态。
- 连接 `server.grpc_target` 调用 `AdminVIPApi.grantVIP`。
- 通过 `xx-admin-secret` 和 `xx-sign` 调用后台接口。

## 构建

```bash
go build -o bin/admin-vip-cli ./cmd/admin_vip_cli
```

## 运行

```bash
./bin/admin-vip-cli
```

工具默认读取 `config.yaml`，也会读取 `SPIDER_SERVER_CONFIG` 指定的配置。只要服务端和工具使用的配置里 `admin.vip_grant_secret` 一致，就不需要额外传 `-secret`。

也可以显式指定配置、gRPC 地址、后台密钥和操作人：

```bash
./bin/admin-vip-cli \
  -config config.yaml \
  -grpc 127.0.0.1:18000 \
  -secret '<admin.vip_grant_secret>' \
  -operator huitailang
```

进入工具后输入账号或朋友展示 ID，例如 `root` 或 `SP000008`。工具会展示用户信息、Apple 登录邮箱、最后进入 App 时间和 VIP 状态，然后选择：

- `1` 开通 1 分钟 VIP
- `2` 开通 7 天 VIP
- `3` 开通一个月 VIP
- `4` 开通三个月 VIP
- `5` 开通一年 VIP
- `6` 开通永久 VIP
- `7` 取消后台开通 VIP
- `0` 返回账号查询

在账号输入行输入 `0`、`q` 或 `exit` 会退出工具。

## 注意

- `admin.vip_grant_secret` 不能为空。
- 本地默认启动使用 `config.yaml`；线上环境本地启动或服务器启动可使用 `SPIDER_SERVER_CONFIG=config.server.yaml` 或对应配置文件。
- 服务端 gRPC 进程需要已经启动。
- 工具只初始化数据库连接，不执行 AutoMigrate。
- 开通动作只写 `user_entitlements` 的后台开通字段，不写 Apple 订单表或支付表。
- 取消动作只清理后台开通字段，不会取消用户通过 Apple 自行购买的 VIP。
