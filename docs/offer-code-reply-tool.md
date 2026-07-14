# Offer Code 回复生成工具

工具读取 App Store Connect 导出的 `OfferCodeOneTimeUseCodes_*.csv`，根据 CSV 中的 1-based 行号生成完整英文回复，并持久化已经生成过的最大 ID。

## 使用

### 浏览器界面（推荐）

在项目根目录启动本地 HTTP 服务：

```bash
go run ./cmd/offer_reply -web
```

然后在浏览器打开：

```text
http://127.0.0.1:8787
```

页面支持输入数字 ID 或完整兑换码、生成回复、一键复制全文，并显示历史最大 ID、建议下一个 ID 和已生成数量。服务默认只监听本机地址，不会对局域网或互联网开放。按 `Ctrl+C` 停止服务。

如果端口被占用，可以换一个端口：

```bash
go run ./cmd/offer_reply -web -addr 127.0.0.1:8788
```

### 命令行模式

在项目根目录运行：

```bash
go run ./cmd/offer_reply
```

把 App Store Connect 导出的 CSV 放在生成器代码同级目录：

```text
cmd/offer_reply/OfferCodeOneTimeUseCodes_xxx.csv
```

工具会优先查找这个目录中的 `OfferCodeOneTimeUseCodes_*.csv`，先显示历史最大 ID，然后等待输入。可以输入：

- 数字 ID，例如 `6`，表示 CSV 第 6 行；
- 完整兑换码，例如 `XXXXXXXXXXXXXXXXXX`。

也可以直接把 ID 放在命令后面：

```bash
go run ./cmd/offer_reply 6
```

需要明确指定 CSV 时：

```bash
go run ./cmd/offer_reply -csv /path/to/OfferCodeOneTimeUseCodes_xxx.csv 6
```

默认状态文件保存在 CSV 旁边，也就是 `cmd/offer_reply/` 目录，文件名为：

```text
OfferCodeOneTimeUseCodes_xxx.offer-reply-state.json
```

因此关闭程序或重启电脑后，工具仍能找到历史最大 ID。同一个 ID 重复生成不会增加最大 ID。状态会绑定 CSV 内容，避免换了一批兑换码后误用旧记录；新 CSV 会自动使用自己对应的状态文件。

可通过 `-state` 指定其他状态文件，也可以设置 `LIFTTAGS_OFFER_CODES_CSV` 环境变量固定 CSV 路径。
