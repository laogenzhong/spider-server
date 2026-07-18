# 动作与计划同步容量限制

服务端通过 `config.yaml`（本地）或 `config.server.yaml`（线上运行包）里的 `workout_data_sync` 配置控制动作、计划、文件夹和训练快照的上传容量与恢复分页。线上配置源是 `config.server.example.yaml`，修改后需要重新构建/部署，或同步修改实际运行的 `config.server.yaml`，并重启服务端后生效。

所有 `*_bytes` 参数都使用字节；`1 MiB = 1,048,576 bytes`。

## 五个参数分别限制什么

| 参数 | 默认值 | 限制对象 | 达到或超过限制时的行为 |
| --- | ---: | --- | --- |
| `gateway_max_request_bytes` | 10 MiB | 网关收到的单个 HTTP `/rpc` 请求体，或单条 WebSocket 消息。它是最外层请求的硬上限，不只计算业务快照字段。 | HTTP 请求在完整读入前被拒绝并返回 `413 Request Entity Too Large`；WebSocket 超限消息不会进入业务处理。用于防止攻击者用超大请求占用网关内存。 |
| `sync_rpc_max_request_bytes` | 4 MiB | 一次 `SyncWorkoutDataSnapshots` RPC 请求 protobuf 的总字节数，包括请求中的全部待上传快照及请求封装字段。 | 整次同步请求被拒绝并返回“同步请求过大”业务错误。客户端应拆成更小批次重试。 |
| `snapshot_max_payload_bytes` | 2 MiB | 写入数据库前，单个动作、计划、文件夹或训练快照序列化后的 protobuf 字节数。 | 该次同步按“快照保存失败”业务错误返回。计划库已按计划/文件夹增量同步，因此它限制单个增量实体，不限制整个计划库的累计大小。 |
| `restore_batch_max_snapshots` | 1000 条 | 登录恢复或增量恢复时，一页最多返回多少条 `workout_data_snapshots`。 | 当前页达到该条数后结束，通过游标继续请求下一页。它防止大量小记录一次性进入内存和响应包。 |
| `restore_batch_target_bytes` | 2 MiB | 恢复分页中，一页内所有快照 `payload` 存储字节数之和的目标值。 | 当前页已有记录、再加入下一条会超过目标值时，下一条留到下一页。它是分页目标而不是单条硬拦截：若某条旧快照本身超过目标值，仍会单独成页返回，所以不会出现“数据永远拿不到”。 |

恢复分页同时受 `restore_batch_max_snapshots` 和 `restore_batch_target_bytes` 控制，任意一个条件先达到就结束当前页。分页按统一游标继续，不使用后段越来越慢的 `OFFSET` 扫描。

## 三层上传限制的关系

一次上传会从外到内依次经过：

```text
HTTP / WebSocket 整体消息
  gateway_max_request_bytes（默认 10 MiB）
    -> SyncWorkoutDataSnapshots 整批快照
       sync_rpc_max_request_bytes（默认 4 MiB）
         -> 单个增量快照实体
            snapshot_max_payload_bytes（默认 2 MiB）
```

建议保持：

```text
gateway_max_request_bytes > sync_rpc_max_request_bytes > snapshot_max_payload_bytes
```

原因如下：

- 网关上限计算整个外层请求，还包含 RPC 封装、认证和其他协议字段，应给业务 protobuf 留出余量。
- RPC 总量上限必须至少容纳一个合法的单实体，并为批量上传保留空间。
- 如果把网关上限调到低于 RPC 上限，实际会先被网关拒绝，RPC 配置中较大的部分不会生效。
- 如果把 RPC 上限调到低于单实体上限，某个实体即使符合单实体规则，也可能因为整批上限先被拒绝，通常不建议这样配置。

## 配置示例

```yaml
workout_data_sync:
  gateway_max_request_bytes: 10485760      # 10 MiB，整个 HTTP/WS 消息
  sync_rpc_max_request_bytes: 4194304      # 4 MiB，一次同步 RPC 的全部快照
  snapshot_max_payload_bytes: 2097152      # 2 MiB，单个增量实体
  restore_batch_max_snapshots: 1000        # 恢复分页最多 1000 条
  restore_batch_target_bytes: 2097152      # 恢复分页目标约 2 MiB
```

例如，一次上传包含 3 MiB 和 1.5 MiB 两个快照：即使整个网关消息没有超过 10 MiB，也会同时触发 4 MiB 的 RPC 总量上限和 2 MiB 的单实体上限，客户端需要拆批，且 3 MiB 的单实体本身仍需缩小。恢复时，如果当前页已有 1.8 MiB，下一条为 0.5 MiB，则当前页会在 1.8 MiB 结束，下一条进入下一页。

## 调整建议

- 正常情况下优先维持默认值，依靠计划增量实体和按字节分页控制传输量，不要用继续放大单实体来替代数据拆分。
- `restore_batch_target_bytes` 可以小于 `snapshot_max_payload_bytes`；合法快照超过分页目标时会单独成页，仍然可以恢复。
- 提高这些值前，还应确认反向代理、负载均衡、gRPC、客户端超时和服务器内存限制，否则应用配置放大后仍可能被更外层基础设施拒绝。
- 降低上限前要考虑数据库中已有旧快照。上传硬上限只影响新写入；恢复目标不会阻断旧大快照，但可能让单页暂时超过目标值。
- 服务启动日志会打印最终生效的五个值，可用于确认线上读取的是哪份配置。
