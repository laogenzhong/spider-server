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

也可以显式指定配置、gRPC 地址、后台密钥和操作人：

```bash
./bin/admin-vip-cli \
  -config config.yaml \
  -grpc 127.0.0.1:18000 \
  -secret '<admin.vip_grant_secret>' \
  -operator huitailang
```

进入工具后输入账号，例如 `sp000001`。工具会展示用户信息和 VIP 状态，然后选择：

- `1` 开通一个月 VIP
- `2` 开通一年 VIP
- `3` 开通永久 VIP
- `0` 退出工具

## 注意

- `admin.vip_grant_secret` 不能为空。
- 服务端 gRPC 进程需要已经启动。
- 工具只初始化数据库连接，不执行 AutoMigrate。
- 开通动作只写 `user_entitlements` 的后台开通字段，不写 Apple 订单表或支付表。
