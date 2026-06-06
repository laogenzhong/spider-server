go mod vendor 只属于本项目的包放在一起
每次 go mod tidy 后都要执行一次 go mod vendor


Go 的 import 路径不是按“文件系统绝对路径”找的，而是按：
go.mod 里的 module 名 + 项目内相对目录

## 本地配置启动

默认读取项目根目录的 `config.yaml`。

如果本机需要覆盖数据库、端口、密钥等配置，可以复制一份本地配置：

```bash
cp config.yaml config.local.yaml
```

启动时指定本地配置文件：

```bash
SPIDER_SERVER_CONFIG=config.local.yaml go run ./cmd
```

`config.local.yaml` 已加入 `.gitignore`，不会被提交。

## 服务器配置

服务器不要直接改本地 `config.yaml`。当前发布流程把 `config.server.example.yaml` 作为线上真实配置源，`./build.sh` 会自动把它打包为服务器运行时读取的 `config.server.yaml`。

服务器启动时指定配置：

```bash
SPIDER_SERVER_CONFIG=config.server.yaml ./spider-server
```

生产配置重点检查：

- `mysql.password`：服务器 MySQL 的 `spider` 用户密码。
- `session.sign_secret`：换成一串足够长的随机字符串。
- `apple_sign_in.private_key_path`：服务器上的 `AuthKey_5LFYA472TZ.p8` 路径。
- `app_store.environment`：沙盒/TestFlight 联调用 `SANDBOX`，正式上架后改为 `PRODUCTION`。
- `app_store.root_certificate_paths`：服务器上的 Apple 根证书路径。
- `client.gateway_base_url`：公网 HTTPS 地址，例如 `https://jjai.top`。

## 公开页面

线上网关提供无需登录的首页，方便审核人员直接访问域名：

```text
https://jjai.top
```

线上网关提供无需登录的技术支持页，供 App Store Connect 填写：

```text
https://jjai.top/support
```

线上网关也提供无需登录的隐私政策页，供 App Store Connect 填写：

```text
https://jjai.top/privacy
```

这些公开页面由 Go 网关直接读取 `public/*.html` 返回，不依赖数据库、Node 或 RPC 服务。

以后只需要替换服务器运行目录中的 HTML 文件即可更新页面，无需重新编译 Go：

```text
public/support.html
public/privacy.html
public/index.html
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

`./build.sh` 会把 `config.server.example.yaml` 同时打包为 `config.server.example.yaml` 和 `config.server.yaml`。包内也会包含 `apple_iap_verifier` 和生产 Node 依赖。服务器仍需要已安装 `node` 运行时，`run.sh` 会在启动前检测。

上传到服务器后：

```bash
tar -xzf spider-server-online-linux-amd64.tar.gz
cd spider-server-online-linux-amd64
./run.sh
```

`run.sh` 会在启动前自动停止当前目录中 `spider-server.pid` 记录的旧进程，然后再启动新进程；正常部署时不需要先手动执行 `stop.sh`。

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
