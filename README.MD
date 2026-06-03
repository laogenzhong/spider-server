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
