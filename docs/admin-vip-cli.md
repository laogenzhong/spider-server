# Admin VIP CLI

`cmd/admin_vip_cli` 是后台开通 VIP 的独立命令行工具。

它会：

- 读取 `config.yaml` 或 `SPIDER_SERVER_CONFIG` 指定的配置。
- 连接 MySQL 查询账号、昵称和当前 VIP 状态。
- 连接 `server.grpc_target` 调用 `AdminVIPApi.grantVIP`。
- 通过 `xx-admin-secret` 和 `xx-sign` 调用后台接口。
- 修改 App 内版本更新配置，包括最新版本、最低支持版本、强制更新开关、普通更新提示开关、App Store URL 和多语言更新文案。

## 构建

```bash
go build -o bin/admin-vip-cli ./cmd/admin_vip_cli
```

## 运行

```bash
./bin/admin-vip-cli
```

配置文件读取规则：

- 普通 `admin-vip-cli`：优先使用 `-config 配置路径`。
- 没有传 `-config` 时：读取 `SPIDER_SERVER_CONFIG` 指定的配置。
- `SPIDER_SERVER_CONFIG` 也没有设置时：读取当前目录的 `config.yaml`。
- 线上包里的 `./admin-vip-cli` 是包装脚本，默认读取同目录的 `config.server.yaml`。

只要服务端和工具使用的配置里 `admin.vip_grant_secret` 一致，就不需要额外传 `-secret`。

线上包会额外包含一个可直接运行的 `admin-vip-cli`：

```bash
cd spider-server-online-linux-amd64
./admin-vip-cli
```

这个线上包内的命令默认使用同目录的 `config.server.yaml`，不需要再手动传 `-config config.server.yaml`。

如果要临时指定其他线上配置，可以直接运行真实二进制：

```bash
./admin-vip-cli.bin -config other.server.yaml
```

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

在账号输入行输入 `u`、`update` 或 `version` 会进入 App 更新配置菜单：

- `1` 输入最新版本。
- `2` 输入最小支持版本。
- `3` 切换强制更新开关。
- `4` 切换更新是否可用开关。
- `5` 输入 App Store URL。
- `6` 配置多语言更新文案：简体中文、繁体中文、英文、日语、韩语。
- `0` 返回账号查询。

版本更新配置只读取数据库表 `app_update_configs`。服务端启动时如果表里没有 iOS 配置记录，会把 `config.server.yaml` 里的 `app_update` 写入数据库一次；如果表里已经有记录，不会覆盖。后续通过 admin 修改数据库记录。

## 注意

- `admin.vip_grant_secret` 不能为空。
- 服务端 `spider-server` 默认读取当前目录的 `config.yaml`，可通过 `SPIDER_SERVER_CONFIG=配置路径` 指定配置文件。
- 线上包的 `run.sh` 默认用 `SPIDER_SERVER_CONFIG=config.server.yaml` 启动服务端。
- 线上包的 `./admin-vip-cli` 默认用同目录的 `config.server.yaml` 启动后台工具。
- 服务端 gRPC 进程需要已经启动。
- 工具只初始化数据库连接，不执行 AutoMigrate。
- 版本更新配置表由服务端启动时 AutoMigrate 创建；部署后先启动一次新版服务端，再使用 admin 修改更新配置。
- 开通动作只写 `user_entitlements` 的后台开通字段，不写 Apple 订单表或支付表。
- 取消动作只清理后台开通字段，不会取消用户通过 Apple 自行购买的 VIP。
