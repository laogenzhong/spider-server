go mod vendor 只属于本项目的包放在一起
每次 go mod tidy 后都要执行一次 go mod vendor


Go 的 import 路径不是按“文件系统绝对路径”找的，而是按：
go.mod 里的 module 名 + 项目内相对目录

## 配置文件和启动规则

服务端和后台工具都从 YAML 配置文件读取数据库、gRPC 地址、密钥、Apple 配置等运行参数。配置文件选择规则如下：

- 服务端 `spider-server`：默认读取当前目录的 `config.yaml`。
- 服务端 `spider-server`：可以通过 `SPIDER_SERVER_CONFIG=配置路径` 指定配置文件。
- 后台 VIP 工具 `admin-vip-cli`：默认读取当前目录的 `config.yaml`。
- 后台 VIP 工具 `admin-vip-cli`：优先使用命令行 `-config 配置路径`，没有传 `-config` 时再读取 `SPIDER_SERVER_CONFIG`，最后才回退到 `config.yaml`。
- 线上包里的 `./admin-vip-cli` 是包装脚本，默认自动执行 `./admin-vip-cli.bin -config config.server.yaml`。

本地开发默认使用项目根目录的 `config.yaml`。如果本机需要覆盖数据库、端口、密钥等配置，可以复制一份本地配置：

```bash
cp config.yaml config.local.yaml
```

本地启动服务端时指定配置文件：

```bash
SPIDER_SERVER_CONFIG=config.local.yaml go run ./cmd
```

本地运行后台 VIP 工具时指定配置文件：

```bash
go run ./cmd/admin_vip_cli -config config.local.yaml
```

`config.local.yaml` 已加入 `.gitignore`，不会被提交。

## 服务器配置

服务器不要直接改本地 `config.yaml`。当前发布流程把 `config.server.example.yaml` 作为线上真实配置源，`./build.sh` 会自动把它打包为服务器运行时读取的 `config.server.yaml`。

线上包启动服务端：

```bash
cd spider-server-online-linux-amd64
./run.sh
```

`run.sh` 在 `SPIDER_SERVER_ENV` 为空或等于 `online` 时，会默认设置：

```bash
SPIDER_SERVER_CONFIG=config.server.yaml
```

如果不通过 `run.sh`，手动启动服务端时需要指定配置：

```bash
SPIDER_SERVER_CONFIG=config.server.yaml ./spider-server
```

线上包运行后台 VIP 工具：

```bash
./admin-vip-cli
```

它会默认读取同目录的 `config.server.yaml`。如果需要临时指定其他配置，也可以直接运行真实二进制：

```bash
./admin-vip-cli.bin -config other.server.yaml
```

生产配置重点检查：

- `mysql.password`：服务器 MySQL 的 `spider` 用户密码。
- `session.sign_secret`：换成一串足够长的随机字符串。
- `admin.vip_grant_secret`：后台开通 VIP 的共享密钥；服务端和 `admin-vip-cli` 使用同一份配置时不需要额外传 `-secret`。
- `apple_sign_in.private_key_path`：服务器上的 `AuthKey_5LFYA472TZ.p8` 路径。
- `app_store.environment`：沙盒/TestFlight 联调用 `SANDBOX`，正式上架后改为 `PRODUCTION`。
- `app_store.root_certificate_paths`：服务器上的 Apple 根证书路径。
- `app_update.ios_latest_version`：App Store 最新版本；高于客户端时会在“我的”页显示轻提示。只用于服务端启动时初始化数据库。
- `app_update.ios_min_supported_version`：最低支持版本；低于该版本的客户端会被强制更新。只用于服务端启动时初始化数据库。
- `client.gateway_base_url`：公网 HTTPS 地址，例如 `https://jjai.top`。

如果线上保留旧的 `config.server.yaml` 而不是使用新包内配置，记得给公开接口白名单补上：

```yaml
auth:
  public_grpc_method_prefixes:
    - "/uc."
    - "/api.AdminVIPApi/"
    - "/api.AppUpdateService/"
```

版本检查示例：

```yaml
app_update:
  ios_latest_version: "1.0.7"
  ios_min_supported_version: "1.0.6"
  force_update_enabled: false
  update_available_enabled: true
  ios_app_store_url: "https://apps.apple.com/app/id6776698752"
  message_zh_hans: "请更新到最新版本后继续使用"
  message_zh_hant: "請更新到最新版本後繼續使用"
  message_en: "Please update to the latest version to continue."
  message_ja: "続行するには最新バージョンにアップデートしてください。"
  message_ko: "계속 사용하려면 최신 버전으로 업데이트해 주세요."
```

这表示低于 `1.0.6` 的客户端强制更新；`1.0.6` 看到 `1.0.7` 时只在“我的”页显示轻提示，不弹窗。

服务端启动后会检查数据库表 `app_update_configs`。如果表里没有 iOS 记录，会把 `config.server.yaml` 的 `app_update` 写入数据库一次；如果表里已经有记录，不会覆盖。运行时版本检查只读取数据库，不再读取配置文件。

线上可通过 admin 工具修改数据库中的版本更新配置：

```bash
./admin-vip-cli
```

在 `Account/UserID>` 输入：

```text
u
```

进入 App 更新配置菜单后可以修改最新版本、最低支持版本、强制更新开关、普通更新提示开关、App Store URL，以及简体中文、繁体中文、英文、日语、韩语文案。

## 公开页面

公开页面现在保留两套可独立部署的静态目录：

```text
public_jjai/      -> https://jjai.top
public_workers/   -> https://lifttags-legal.laogenzhong.workers.dev
```

`public` 目录仍作为服务器网关备用入口使用。
`public` 和 `public_jjai` 页脚包含 ICP 备案号：京ICP备2026034739号-2A。

`jjai.top` 提供无需登录的首页，方便审核人员直接访问：

```text
https://jjai.top/
```

`jjai.top` 提供无需登录的技术支持页，供 App Store Connect 填写：

```text
https://jjai.top/support.html
```

`jjai.top` 也提供无需登录的隐私政策页，供 App Store Connect 填写：

```text
https://jjai.top/privacy.html
```

`jjai.top` 也提供无需登录的使用条款页，供 App Store Connect 的 EULA 链接填写：

```text
https://jjai.top/terms/
```

服务器网关也保留同样的公开页面路由作为备用入口。公开页面都是纯 HTML 文件，不依赖数据库、Node 或 RPC 服务。

以后只需要替换服务器运行目录中的 HTML 文件即可更新页面，无需重新编译 Go：

```text
public/support.html
public/privacy.html
public/index.html
public/terms/index.html
```

替换后直接刷新浏览器即可生效。

## Linux 线上包构建

在本机项目根目录直接执行：

```bash
./build.sh
```

脚本会生成 Linux x86_64 线上包：

```bash
bin/spider-server-online-linux-amd64.tar.gz
```

`./build.sh` 会把 `config.server.example.yaml` 同时打包为 `config.server.example.yaml` 和 `config.server.yaml`。包内也会包含 `admin-vip-cli`、`apple_iap_verifier` 和生产 Node 依赖。服务器仍需要已安装 `node` 运行时，`run.sh` 会在启动前检测。

上传到服务器后：

```bash
tar -xzf spider-server-online-linux-amd64.tar.gz
cd spider-server-online-linux-amd64
./run.sh
```

`run.sh` 会在启动前自动停止当前目录中 `spider-server.pid` 记录的旧进程，然后再启动新进程；正常部署时不需要先手动执行 `stop.sh`。

线上包内可以直接运行后台 VIP 工具，默认读取同目录的 `config.server.yaml`：

```bash
./admin-vip-cli
```

## 同步线上包

构建完成后，可以直接把 tar 包同步到服务器目录：

```bash
./sync.sh
```

默认上传到：

```bash
root@spiderapi.jjai.top:/root/app/spider/api/
```

如需临时改服务器或目录：

```bash
REMOTE_HOST=example.com REMOTE_DIR=/root/app/spider/api ./sync.sh
```
