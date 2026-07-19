# 客户端同步失败归档

`client_sync_failures` 保存客户端队列连续三次业务失败后、需要人工排查或补偿的任务。归档接口为：

`/api.ClientSyncFailureService/ArchiveClientSyncFailure`

## 核心字段

| 字段 | 说明 |
|---|---|
| `uid` | 服务端从已认证 Session 获取的请求用户，客户端不能指定 |
| `client_task_id` | 客户端持久化任务 ID；与 `uid` 组成幂等唯一键 |
| `queue_name` | 来源队列 |
| `original_rpc_path` | 原本需要请求的真实 RPC 接口 |
| `original_request_body` | 原始 protobuf/队列请求数据，用于精确重放 |
| `request_data_json` | 面向人工处理的结构化请求数据 |
| `business_code` / `business_message` | 最后一次失败的业务错误码和说明 |
| `attempt_count` | 客户端尝试次数，当前应为 3 |
| `client_created_at` | 原任务创建时间，Unix 毫秒 |
| `last_failed_at` | 第三次/最后一次请求失败时间，Unix 毫秒 |
| `app_version` | 产生任务的客户端版本 |
| `status` | 人工处理状态，默认 `pending` |
| `resolved_at` / `resolved_by` / `resolution_note` | 人工修复完成时间、处理人和说明 |

服务端优先保存客户端提供的合法业务 JSON。没有 JSON 时，会根据 `original_rpc_path` 查找 protobuf 方法描述符并解码原始请求体；无法识别的旧任务至少保存 `protobuf_base64`，不会丢失原始数据。

## 人工处理原则

1. 先按 `status = 'pending'`、`last_failed_at` 查询任务。
2. 核对 `uid`、目标接口、业务错误码和 `request_data_json`。
3. 使用 `original_request_body` 或结构化数据执行幂等修复，避免重复创建数据。
4. 完成后将 `status` 更新为 `resolved`，并填写 `resolved_at`、`resolved_by`、`resolution_note`。

Admin 控制台“商业与系统 → 丢弃任务”默认仅显示 `pending` 任务，支持按最后失败日期、UID、任务 ID、接口或错误码查询。展开行可以查看结构化请求数据；页面不会下发原始二进制请求体。点击“标记已处理”调用：

`POST /admin-console/client-sync-failures/:id/resolve`

该操作只更新处理状态和审计字段，不会自动重放请求。列表接口为 `GET /admin-console/client-sync-failures`，分页固定按 `last_failed_at DESC, id DESC` 排序。

归档请求体可能包含用户业务数据。不要把 `original_request_body` 或 `request_data_json` 输出到普通请求日志；当前拦截器仅记录 protobuf `bytes` 长度。
