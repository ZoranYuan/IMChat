# Room Summary Agent gRPC 对接方案

## 1. 目标与范围

本方案用于把现有 IM 后端作为 gRPC client，对接 `room_agent.v1.RoomSummaryAgent/RunSummary` 服务。

Agent 本身只允许被后端通过 gRPC 调用，前端不能直接连接 Agent。`START`、`RETRY`、`CANCEL` 都是后端发给 Agent 的 gRPC action，不要求前端通过 WebSocket 发送。前端主要订阅摘要任务状态和最终结果，因此使用 SSE 作为摘要通知通道更合适。

摘要功能应作为一个独立的后端应用用例接入，不要直接塞进消息发送、Kafka 投递或现有 IM WebSocket fanout 主链路。现有 WebSocket 继续负责聊天消息、已读回执等 IM 实时事件；摘要结果通过独立 SSE 流推送，Agent 的实际调用仍然走后端 gRPC。

目标包括：

- 后端校验房间和用户权限；
- 根据本地摘要游标组装增量消息；
- 通过 gRPC 调用 Agent 的 `START`、`RETRY`、`CANCEL`；
- 通过 `ResumeSummaryState` 恢复 SSE 断线或后端重启后的摘要运行；
- 持久化摘要任务、每次 RPC 请求和摘要结果；
- 保证同一个 `summary_run_id` 不并发发送多个请求；
- 只有摘要成功后才推进摘要游标，避免消息被错误跳过；
- 对 gRPC 状态码、Agent 返回状态和网络重试进行区分处理。

## 2. 当前后端情况

当前项目已经具备以下基础：

- 消息实体包含 `messageId`、`conversationId`、`senderId`、`seq`、`type`、`content`、`status`、`sendTime`；
- 消息仓储已经支持按会话和 `seq` 查询，但现有 `ListAfterSeq` 没有上界参数，并且存在业务过滤逻辑，不建议直接用于摘要；
- 房间仓储和房间成员仓储可以用于校验房间状态及用户是否属于房间；
- 后端通过 `main.go` 完成依赖注入和服务生命周期管理；
- 项目已经使用 protobuf，但当前尚未发现 Agent gRPC client、摘要任务模块或摘要相关数据库表；
- 当前 `go.mod` 还需要增加 `google.golang.org/grpc` 依赖。

因此这次对接应新增一个独立的摘要模块，并复用现有的房间、成员、消息仓储能力。

## 3. 推荐架构

```text
后端内部业务调用
        |
        v
RoomSummaryApplication
        |
        +-- 房间/成员权限校验
        +-- 摘要游标读取
        +-- 查询增量消息
        +-- summary_run 状态机
        +-- summary_request 审计
        |
        v
RoomSummaryAgent 应用端口
        |
        v
gRPC Client Adapter
        |
        v
room_agent.v1.RoomSummaryAgent
  ├─ RunSummary(START/RETRY/CANCEL)
  └─ ResumeSummaryState
        |
        v
Agent 响应 -> 摘要任务/事件持久化 -> SSE Hub
                                      |
                                      v
                                  前端摘要状态
```

建议分层如下：

```text
internal/application/summary
    摘要用例、状态机、权限校验、游标推进

internal/application/ports
    Agent client 端口（RunSummary/ResumeSummaryState）、摘要仓储端口、消息查询端口

internal/infrastructure/agent/grpc
    protobuf 生成代码、gRPC client、协议字段映射

internal/infrastructure/persistence/mysql
    摘要任务、请求记录、摘要游标的 MySQL 实现
```

应用层不要直接依赖 protobuf 生成类型。应用层使用自己的请求和结果结构，gRPC adapter 负责把应用结构转换成 `summary.proto` 的消息。

### 3.1 前端实时通道：使用 SSE

本项目的摘要前端链路建议与现有 IM WebSocket 分开：

```text
后端内部业务创建摘要任务
        |
        v
SummaryWorker 调用 Agent gRPC
        |
        v
持久化任务状态和摘要事件
        |
        v
用户的 SSE 订阅流
        |
        v
前端更新摘要状态、展示结果或展示重试/取消按钮
```

这里 SSE 只负责服务端到前端的状态和结果通知；`START`、`RETRY`、`CANCEL` 仍然是后端到 Agent 的 gRPC 请求。前端不直接理解或连接 Agent。

Agent 当前的 `RunSummary` 是一次请求返回一个 `SummaryResponse`，并不是 server-streaming gRPC。因此 SSE 推送的是后端任务事件，例如：

- `RUNNING`：任务已经入队或正在调用 Agent；
- `SUCCEEDED`：包含摘要片段和引用消息；
- `WAITING_USER_DECISION`：包含 `RetryRequired`；
- `FAILED`：包含稳定错误码和可展示错误信息。

如果未来 Agent 改成真正的流式输出，后端可以把中间结果转换成多个 SSE 事件；当前不需要为了 SSE 修改 gRPC 协议。

### 3.1.1 SSE 断线续传设计

SSE 的自动重连和 `Last-Event-ID` 很适合摘要结果通知，但必须由后端提供可恢复的事件来源。不能只依赖进程内 channel 或 Redis Pub/Sub，因为它们无法保证客户端断线期间的事件仍然存在。

建议 SSE 事件使用全局递增且持久化的 ID（例如 MySQL 自增 bigint）：

```text
event_id = 10001, 10002, 10003, ...
```

事件示例：

```text
id: 10003
event: room_summary_status
data: {"summaryRunId":"run-001","status":"SUCCEEDED"}
```

断线重连时：

1. 浏览器自动携带 `Last-Event-ID`；
2. 后端根据用户身份和 event_id 查询未发送事件；
3. 先补发断线期间的事件；
4. 再继续监听新的摘要事件。

如果事件保留周期已经过期，后端至少要根据 `room_summary_runs` 发送当前任务快照，不能让前端因为错过事件而永久停留在旧状态。对于仍处于 `RUNNING` 的任务，后端还应调用 `ResumeSummaryState` 获取 Agent 当前状态；不能通过重新 `START` 恢复。

### 3.1.2 多实例和 SSE 推送

用户的 SSE 连接可能在实例 A，而 SummaryWorker 可能在实例 B。建议：

- 数据库保存任务最终状态和可恢复事件；
- Redis Pub/Sub 或 Redis Streams 用于低延迟通知其他实例；
- 每个实例只向自己持有的 SSE connection 推送；
- 客户端重连时以数据库事件/任务状态补偿，不依赖 Pub/Sub 补历史。

SSE 客户端需要心跳注释，例如 `: ping`，并在连接关闭、请求取消或客户端发送速度异常时释放本地订阅资源。

### 3.2 为什么当前不选摘要 WebSocket

WebSocket 当然可以实现同样功能，但当前摘要结果是单向通知，使用 WebSocket 会把摘要事件和已有聊天双向协议耦合在一起，并且需要额外定义摘要操作帧、状态帧和断线恢复协议。

当前项目保留 WebSocket 处理 IM 消息和已读回执，摘要使用 SSE，可以让两条链路职责更清晰。若未来摘要需要前端持续发送交互指令，再单独增加控制通道即可，不影响 SSE 的结果订阅。

## 4. Proto 与 gRPC Client

### 4.1 Proto 文件和代码生成

协议文件固定为：

```text
proto/room_agent/v1/summary.proto
```

需要确认 proto 中存在正确的 `go_package`，例如：

```proto
option go_package = "github.com/<module>/backend/internal/infrastructure/agent/pb;agentpb";
```

生成 Go 代码时至少需要：

- `protoc-gen-go`；
- `protoc-gen-go-grpc`；
- `google.golang.org/protobuf`；
- `google.golang.org/grpc`。

建议把生成代码放在类似下面的位置：

```text
backend/internal/infrastructure/agent/pb/summary.pb.go
backend/internal/infrastructure/agent/pb/summary_grpc.pb.go
```

具体生成命令应以仓库实际 Go module 名称和 proto 的 `go_package` 为准，不能在没有确认 module path 的情况下硬编码导入路径。

### 4.2 Client 端口

应用层定义与 gRPC 无关的端口，例如：

```go
type RoomSummaryAgent interface {
    RunSummary(ctx context.Context, req RunSummaryRequest) (RunSummaryResponse, error)
    ResumeSummaryState(ctx context.Context, req ResumeSummaryStateRequest) (RunSummaryResponse, error)
}
```

`RunSummaryRequest` 至少应包含：

- `RequestID`；
- `SummaryRunID`；
- `RoomID`；
- `UserID`；
- `Action`；
- `RoomMessages`。

`ResumeSummaryStateRequest` 至少包含：

- `RequestID`；
- `SummaryRunID`；
- `RoomID`；
- `UserID`。

Resume 请求不包含 `RoomMessages`，也不携带用户对中断的选择。它只用于根据 `summary_run_id` 恢复或查询 Agent 当前运行状态。

gRPC adapter 内部再转换为：

```text
agentpb.SummaryRequest
agentpb.ResumeSummaryStateRequest
```

这样做的好处是应用层不会被 protobuf 包、gRPC status code 和连接细节污染，测试时也可以直接使用 mock Agent port。

### 4.3 Client 生命周期

后端进程启动时创建一个 gRPC connection 和一个 Agent client，并通过依赖注入传给摘要应用服务：

```text
main.go
  -> 创建 gRPC connection
  -> 创建 RoomSummaryAgent adapter
  -> 创建 RoomSummaryApplication
  -> 注入到内部调用方
```

进程退出时关闭 connection。不要在每次摘要调用中重新建立 gRPC 连接。

每次 RPC 都应创建带超时的 context，不能使用无期限的后台调用：

```go
rpcCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
defer cancel()
```

### 4.4 Agent 状态和中断动作映射

Agent 返回的 `SummaryStatus` 以协议定义为准：

| Agent 状态 | 后端本地状态 | 处理方式 |
| --- | --- | --- |
| `SUMMARY_STATUS_RUNNING` | `RUNNING` | 保存运行中状态，后续通过 `ResumeSummaryState` 继续或查询 |
| `SUMMARY_STATUS_SUCCEEDED` | `SUCCEEDED` | 保存摘要结果，并推进本轮摘要游标 |
| `SUMMARY_STATUS_WAITING_USER_DECISION` | `WAITING_USER_DECISION` | 保存 `retry_required`，等待用户选择 |
| `SUMMARY_STATUS_FAILED` | `FAILED` | 保存失败信息，不推进游标 |

`RetryRequired.allowed_actions` 表示用户对 interrupt 的选择，应映射为 `INTERRUPT_ACTION_RETRY` 或 `INTERRUPT_ACTION_CANCEL`。真正发送给 Agent 时，仍然使用 `SUMMARY_ACTION_RETRY` 或 `SUMMARY_ACTION_CANCEL`。

`ResumeSummaryState` 的行为由 Agent 根据 Checkpoint 决定：Checkpoint 不存在则返回错误；Graph 已结束则返回最终状态；Graph 停在 interrupt 则返回 `WAITING_USER_DECISION`；存在未完成节点且没有 interrupt 时继续执行 Graph。因此后端不能只把 Resume 当作查询接口，也不能把 Resume 返回的 `RUNNING` 误判为失败。

## 5. 配置设计

在现有配置结构中新增独立的 Agent 配置段，建议包含：

```yaml
room_summary_agent:
  enabled: true
  endpoint: "agent-service:9000"
  request_timeout: 30s
  max_messages: 500
  max_message_bytes: 1048576
  insecure: false
  tls_server_name: ""
  auth_token: ""
```

配置字段建议包括：

- `enabled`：是否启用摘要 Agent；
- `endpoint`：Agent gRPC 地址；
- `request_timeout`：单次 RPC 超时时间；
- `max_messages`：一次 START 最大消息数量；
- `max_message_bytes`：一次 START 最大请求体大小；
- TLS 证书、server name 或认证 metadata 配置；
- 连接保活参数（如果部署环境需要）。

协议中没有明确 TLS、认证方式和服务发现方式，这些需要在实现前确认。尤其不能把生产环境的认证 token 写死在代码中。

## 6. 数据库设计

建议新增四张表：摘要任务表、RPC 请求记录表、摘要游标表、摘要事件表。

### 6.1 `room_summary_runs`

用于记录一次摘要任务的生命周期和最终结果。

建议字段：

| 字段 | 说明 |
| --- | --- |
| `summary_run_id` | 摘要任务 ID，主键 |
| `room_id` | 房间 ID |
| `user_id` | 发起摘要的用户 ID |
| `status` | `PENDING`、`RUNNING`、`SUCCEEDED`、`WAITING_USER_DECISION`、`FAILED` |
| `after_seq` | 本轮摘要输入的起始游标，不包含该 seq |
| `until_seq` | 本轮摘要输入的快照上界 |
| `summary_content` | Agent 成功返回的摘要 JSON |
| `retry_required` | Agent 返回的人工决策信息 JSON |
| `last_request_id` | 最近一次 RPC 请求 ID |
| `active_request_id` | 当前正在执行的请求 ID |
| `version` | 乐观锁版本号 |
| `last_error_code` | 最近一次错误码 |
| `last_error_message` | 最近一次错误信息 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |

建议建立以下索引：

- `PRIMARY KEY (summary_run_id)`；
- `(room_id, user_id, created_at)`；
- 用于限制同一用户同一房间的活动任务的索引或唯一约束。

`active_request_id` 和 `version` 用于多实例下的任务抢占和并发控制。任务进入执行阶段前，需要通过数据库条件更新或行锁把它从 `PENDING` 变为 `RUNNING`。

是否允许同一个用户在同一个房间同时存在多个摘要任务，需要产品明确。推荐默认只允许一个活动任务，避免同一摘要游标被多个任务同时推进。

### 6.2 `room_summary_requests`

用于记录每次 gRPC 请求，而不是只记录一次摘要任务。

建议字段：

| 字段 | 说明 |
| --- | --- |
| `request_id` | 每次 RPC 请求的唯一 ID，主键或唯一索引 |
| `summary_run_id` | 所属摘要任务 |
| `rpc_method` | `RunSummary` 或 `ResumeSummaryState` |
| `action` | `START`、`RETRY`、`CANCEL`、`RESUME` |
| `request_status` | `PENDING`、`SUCCEEDED`、`FAILED`、`UNKNOWN` |
| `grpc_code` | gRPC status code |
| `agent_status` | Agent 返回的 SummaryStatus |
| `started_at` | RPC 开始时间 |
| `finished_at` | RPC 结束时间 |
| `error_message` | 错误摘要 |

`request_id` 必须唯一，便于日志、Trace、幂等审计和排查重复调用。

默认不建议把完整房间消息内容写入请求审计表，避免数据库放大和敏感信息泄漏；如果确实需要重放，应单独设计受控的输入快照表或使用消息表按 `after_seq/until_seq` 重建。

### 6.3 `room_summary_cursors`

用于保存某个用户在某个房间已经成功摘要到哪里。

建议字段：

| 字段 | 说明 |
| --- | --- |
| `room_id` | 房间 ID |
| `user_id` | 用户 ID |
| `last_summarized_seq` | 最近一次成功摘要的最大 seq |
| `last_summary_run_id` | 最近一次成功任务 ID |
| `updated_at` | 更新时间 |

主键建议为：

```text
(room_id, user_id)
```

游标推进必须和摘要任务成功状态更新放在同一个数据库事务中。`RUNNING`、`WAITING_USER_DECISION`、传输失败、Agent `FAILED`、`CANCEL` 都不能推进游标。

### 6.4 `room_summary_events`

用于保存 SSE 断线重连时需要补发的事件。`request_id` 是 Agent gRPC 请求 ID，不能直接拿来当 SSE event ID；SSE 应该有自己独立的、递增且持久化的事件 ID。

建议字段：

| 字段 | 说明 |
| --- | --- |
| `event_id` | SSE 事件 ID，建议使用数据库自增 bigint，主键 |
| `user_id` | SSE 订阅所属用户 |
| `room_id` | 房间 ID |
| `summary_run_id` | 摘要任务 ID |
| `event_type` | `summary_running`、`summary_succeeded` 等 |
| `payload` | 推给前端的 JSON 内容 |
| `created_at` | 事件创建时间 |

建议建立以下索引：

- `PRIMARY KEY (event_id)`；
- `(user_id, event_id)`，用于根据 `Last-Event-ID` 查询该用户后续事件；
- `(summary_run_id, event_id)`，用于查询某个摘要任务的事件。

SSE 重连时执行类似查询：

```sql
SELECT event_id, event_type, payload
FROM room_summary_events
WHERE user_id = ? AND event_id > ?
ORDER BY event_id ASC
LIMIT ?;
```

建议在同一个事务中完成“更新摘要任务状态 + 写入摘要事件”。例如 Agent 返回 `SUCCEEDED` 时，同时更新 `room_summary_runs`、`room_summary_cursors` 和插入 `summary_succeeded` 事件，避免状态已经成功但 SSE 没有可恢复事件。

Redis Pub/Sub 或 Redis Streams 可以作为低延迟通知层：事件写入 MySQL 后发布一个“有新事件”的通知，SSE Hub 收到后从 MySQL 读取事件并推送。Redis Pub/Sub 本身不负责断线补发，不能替代这张事件表。

如果事件表设置了保留期限并删除了旧事件，服务端发现 `Last-Event-ID` 已经早于可恢复范围时，应发送当前任务快照，而不是假装完成了精确补发。

## 7. 消息查询与快照范围

### 7.1 不直接复用现有 `ListAfterSeq`

现有 `ListAfterSeq` 不适合作为摘要专用查询，原因包括：

- 没有明确的 `untilSeq` 上界；
- 当前实现包含特定消息过滤逻辑，不能默认代表摘要所需的完整消息集合；
- 不能控制单次摘要最大消息数和最大请求体；
- 摘要需要可审计、可重建的输入范围。

建议新增独立仓储方法：

```go
ListMessagesForSummary(
    ctx context.Context,
    roomID string,
    afterSeq int64,
    untilSeq int64,
    limit int,
) ([]*messageentity.Message, error)
```

SQL 语义应明确为：

```text
room_id = roomID
AND seq > afterSeq
AND seq <= untilSeq
ORDER BY seq ASC
LIMIT limit
```

还要确认是否应该包含撤回消息。协议要求 Agent 自动过滤 `status == "withdraw"`，因此后端可以把撤回消息一并传给 Agent，保留消息序列完整性；不要在后端静默删除后造成摘要输入无法解释。

### 7.2 为什么摘要需要 `untilSeq`

这里的 `untilSeq` 是本轮摘要输入的快照上界，不是 `/sync` 的分页参数。

开始摘要时先读取当前房间的最大 seq，记为 `untilSeq`，再查询：

```text
(afterSeq, untilSeq]
```

这样在 Agent 处理期间新产生的消息不会混入本轮任务，也不会因为本轮任务延迟而被错误标记为已摘要。下一轮 START 再从本轮成功的 `untilSeq` 继续读取。

如果消息数量超过 `max_messages`，不能简单截断后仍然把 `untilSeq` 全部推进。推荐方式是：

1. 读取游标后的第一批消息；
2. 将最后一条实际取到的消息 seq 作为本轮 `untilSeq`；
3. 下一轮继续从该 seq 读取。

这样可以确保每条消息都不会被游标跳过。

## 8. RoomMessage 映射规则

应用层实体到 protobuf 的映射建议如下：

| protobuf 字段 | 来源与规则 |
| --- | --- |
| `room_id` | 使用经过权限校验的 `roomID`，不得信任消息自身携带的房间 ID |
| `message_id` | `Message.MessageId` |
| `sender_id` | `Message.SenderId` |
| `seq` | `Message.Seq` |
| `type` | 消息类型转换为 `int32` |
| `content` | `Message.Content` |
| `status` | 正常消息传协议约定值，撤回消息传 `"withdraw"` |
| `send_time` | `time.UnixMilli(Message.SendTime)` 转为 `timestamppb.Timestamp` |

当前项目的 `sendTime` 是毫秒时间戳。映射时应使用：

```go
timestamppb.New(time.UnixMilli(msg.SendTime))
```

如果 `sendTime <= 0`，应在调用 Agent 前判定为本地数据异常并记录错误；协议明确规定缺少 `send_time` 时 Agent 返回 `INVALID_ARGUMENT`，不要等请求发出后才发现。

Agent 会按 `seq` 排序并按 `message_id` 去重，但后端仍然应该：

- 查询固定房间范围；
- 以 `seq ASC` 查询；
- 校验 message ID 非空；
- 校验消息不超过配置的数量和大小上限；
- 校验消息 `room_id` 与请求 scope 一致。

## 9. START 调用流程

```text
内部业务触发 START
        |
        v
校验 room 存在且可用
        |
        v
校验 user 是房间成员
        |
        v
读取 room/user 摘要游标 afterSeq
        |
        v
生成 summary_run_id 和 request_id
        |
        v
事务写入 summary_run(PENDING) + summary_request(PENDING)
        |
        v
Worker 抢占任务并读取当前最大 seq，形成 untilSeq
        |
        v
查询 (afterSeq, untilSeq] 的消息
        |
        v
更新任务为 RUNNING，调用 Agent START
        |
        +--> SUCCEEDED：事务保存摘要并推进游标
        |
        +--> WAITING_USER_DECISION：保存 retry_required，不推进游标

        +--> RUNNING：保存运行中状态，后续使用 ResumeSummaryState 查询/继续
        |
        +--> FAILED：保存失败信息，不推进游标
        |
        +--> gRPC 错误：保存传输/协议错误，不推进游标
```

如果 START 由前端控制通道或其他内部业务触发，入口只负责校验、创建 `PENDING` 任务并返回任务已接受结果；消息查询和 gRPC 调用由 Worker 执行。Worker 抢占任务后应锁定对应的 `(room_id, user_id)` 摘要游标，读取 `afterSeq` 和当前最大 seq，再把本轮实际使用的范围写入任务记录。

`room_messages` 可以为空，不能把空消息误判为参数错误。协议规定 Agent 会返回空摘要并标记为 `SUCCEEDED`；此时任务可以正常结束，摘要游标保持不变。是否在本地提前跳过空消息属于优化选择，但如果需要完整记录一次摘要运行，建议仍然发送 START 并保存 Agent 返回结果。

### 9.1 ID 规则

START 必须满足：

- 生成新的 `summary_run_id`；
- 生成新的 `request_id`；
- `scope.room_id` 使用经过校验的房间 ID；
- `scope.user_id` 使用当前后端调用上下文中的用户 ID；
- `action = SUMMARY_ACTION_START`；
- `room_messages` 为本轮增量消息。

建议复用项目现有的 ID 生成器，保证 ID 风格与现有业务一致。

### 9.2 成功后的事务

Agent 返回 `SUCCEEDED` 后，应在一个数据库事务中完成：

1. 更新 `room_summary_runs.status = SUCCEEDED`；
2. 保存 `summary.summary_content`；
3. 更新 `room_summary_requests` 为成功，并保存 Agent 状态；
4. 将 `room_summary_cursors.last_summarized_seq` 更新为本轮 `untilSeq`；
5. 更新 `last_summary_run_id`。

如果这几个操作不能原子完成，就可能出现“结果已经保存但游标没推进”或“游标推进但摘要结果没保存”的不一致。

## 10. ResumeSummaryState 调用流程

`ResumeSummaryState` 专门用于 SSE 断线、后端重启或任务恢复，不代表用户选择重试。

触发恢复时：

1. 根据本地 `summary_run_id` 加载未结束的摘要任务；
2. 校验当前用户与任务的 `user_id` 一致；
3. 生成新的 `request_id`；
4. 使用原来的 `summary_run_id` 和原来的 scope；
5. 不携带 `room_messages`；
6. 调用 `ResumeSummaryState`；
7. 根据 Agent 返回状态更新本地任务并生成 SSE 事件。

请求结构必须是：

```text
request_id：新生成
summary_run_id：保持不变
scope：保持不变
room_messages：不存在
```

响应处理规则：

- `SUCCEEDED`：保存最终摘要，并将本轮 START 保存的 `untilSeq` 推进到摘要游标；
- `WAITING_USER_DECISION`：保存 `retry_required`，等待用户选择 RETRY 或 CANCEL；
- `RUNNING`：任务仍在执行或存在未完成节点，保持本地 `RUNNING`，按退避策略再次 Resume；
- `FAILED`：保存失败状态，不推进摘要游标；
- Checkpoint 不存在：记录 Agent 错误，不能重新发送 START 试图“修复”断点。

特别注意：

- SSE 断线不发送 START；
- SSE 断线不发送 RETRY；
- Resume 不会重置 Agent 的自动修复次数；
- Resume 不创建新的 `summary_run_id`；
- Resume 请求也要写入 `room_summary_requests`，其 `rpc_method` 为 `ResumeSummaryState`，内部 action 可记为 `RESUME`。

## 11. RETRY 调用流程

用户或内部业务要求继续重试时：

1. 根据 `summary_run_id` 读取任务；
2. 校验调用用户与任务的 `user_id` 一致；
3. 校验任务仍处于 `WAITING_USER_DECISION`；
4. 校验 Agent 返回的 `allowed_actions` 包含 `INTERRUPT_ACTION_RETRY`；
5. 生成新的 `request_id`；
6. 使用原来的 `summary_run_id` 和原来的 scope；
7. `room_messages` 必须为空；
8. 写入新的 `summary_request`；
9. 调用 Agent `RETRY`。

控制入口不应同步等待 RETRY 的 gRPC 响应。它只创建一个新的 `PENDING` request 并返回已接受结果，由 Worker 执行 RETRY；Worker 必须先锁定原 `summary_run_id`，确保同一个任务不会同时执行 RETRY 和 CANCEL。

请求结构的核心约束是：

```text
summary_run_id：保持不变
request_id：重新生成
scope：保持不变
action：RETRY
room_messages：为空
```

如果 RETRY 返回 `SUCCEEDED`，仍然要推进原 START 任务保存的 `untilSeq`，不能使用 RETRY 时重新读取的当前最大 seq。RETRY 返回 `WAITING_USER_DECISION` 时更新新的 `retry_required` 信息，游标继续保持不变。若返回 `RUNNING`，任务继续保持运行中，后续使用 `ResumeSummaryState`，不能把它当作用户再次 RETRY。

## 12. CANCEL 调用流程

放弃任务时：

1. 加载并校验 `summary_run_id`；
2. 校验用户和 scope；
3. 校验 Agent 返回的 `allowed_actions` 包含 `INTERRUPT_ACTION_CANCEL`；
4. 生成新的 `request_id`；
5. 使用相同的 `summary_run_id` 和 scope；
6. `room_messages` 为空；
7. 调用 Agent `CANCEL`；
8. 将本地任务最终标记为 `FAILED`；
9. 不推进摘要游标。

CANCEL 也建议进入同一个持久化任务队列，由 Worker 调用 Agent。若 Agent 返回成功，任务才标记为最终 `FAILED`；如果调用在网络层未知失败，不能直接假设 Agent 已经完成 CANCEL，应记录 `UNKNOWN` 并按补偿策略处理。

核心请求结构为：

```text
summary_run_id：保持不变
request_id：重新生成
scope：保持不变
action：CANCEL
room_messages：为空
```

## 13. 摘要任务状态机

```text
PENDING
  |
  v
RUNNING ---------> SUCCEEDED
  |
  +--------------> FAILED
  |
  +--------------> WAITING_USER_DECISION
                         |
                         +-- RETRY --> RUNNING
                         |
                         +-- CANCEL -> FAILED

RUNNING -- ResumeSummaryState --> RUNNING / SUCCEEDED / WAITING_USER_DECISION / FAILED
```

状态约束：

- 新一轮摘要必须生成新的 `summary_run_id`；
- 同一摘要任务的 RETRY、CANCEL 必须复用原 `summary_run_id`；
- SSE 重连或任务恢复调用 `ResumeSummaryState` 时也复用原 `summary_run_id`；
- 每次实际发送的逻辑 RPC 都生成新的 `request_id`；
- 同一个 `summary_run_id` 不允许并发存在两个活动请求；
- 只有 `SUCCEEDED` 可以推进摘要游标；
- `WAITING_USER_DECISION` 必须保存完整的 `RetryRequired`；
- `CANCEL` 最终变为 `FAILED`；
- 已经是终态的任务不能再次 RETRY 或 CANCEL，除非协议另行规定。

并发控制建议使用数据库行锁或乐观锁：读取任务时锁定任务行，并检查 `active_request_id` 或版本号。不能只依赖进程内 mutex，因为后端可能有多个实例。

## 14. gRPC 错误处理

gRPC adapter 应将 status code 转换为应用层明确的错误类型，同时在 `room_summary_requests` 中记录原始 code。

| gRPC code | 应用层处理建议 |
| --- | --- |
| `INVALID_ARGUMENT` | 输入或任务状态不合法，通常终止当前请求，不自动重试 |
| `PERMISSION_DENIED` | scope 不匹配，终止当前请求并记录安全日志 |
| `NOT_FOUND` 或 Checkpoint 不存在 | 记录任务与 Agent 状态不一致，不能直接重新发送 START |
| `DEADLINE_EXCEEDED` | 记录为传输失败，是否重试要根据请求幂等协议决定 |
| `UNAVAILABLE` | Agent 不可用，进入可控的后台重试或失败状态 |
| `INTERNAL` | 记录错误并按运营策略重试，不能无限重试 |
| 未知 code | 按不可安全重试处理并告警 |

`WAITING_USER_DECISION` 是正常的业务响应状态，不是 gRPC 错误。只有 Agent 正常返回该状态时，才保存 `RetryRequired` 并等待后续 RETRY/CANCEL 决策。`RUNNING` 同样是正常响应状态，表示 Agent 仍在执行或 Checkpoint 中存在未完成节点。

### 14.1 传输重试的关键问题

协议同时要求：

- 每次 RPC 请求都要使用新的 `request_id`；
- `request_id` 用于日志、Trace、幂等。

如果后端在发送 START 后遇到网络超时，可能不知道 Agent 是否已经创建了 Checkpoint。此时不能直接再次发送 START，因为 START 要求 `summary_run_id` 不能已存在。应保留原 `summary_run_id`，使用新的 `request_id` 调用 `ResumeSummaryState` 做状态恢复；如果 Checkpoint 不存在，再进入明确的人工或补偿处理。

因此在正式实现自动重试前，需要和 Agent 方确认以下一种机制：

1. `ResumeSummaryState` 能够覆盖 START、RETRY、CANCEL 的未知结果恢复；
2. Agent 明确 Checkpoint 不存在时后端的补偿处理方式；
3. Agent 明确 `ResumeSummaryState` 自身超时后的安全重试规则；
4. 在协议未明确前，Resume 超时只记录 `UNKNOWN`，交给人工或补偿任务处理，不盲目重复用户动作。

这是生产环境必须解决的问题，否则“客户端重试”可能变成“重复摘要任务”。

## 15. 日志、Trace 与监控

每次调用至少记录以下结构化字段：

- `request_id`；
- `summary_run_id`；
- `room_id`；
- `user_id`，必要时脱敏；
- `rpc_method`；
- `action`；
- `after_seq`、`until_seq`；
- 消息数量和请求字节数；
- RPC 耗时；
- gRPC status code；
- Agent `SummaryStatus`；
- 最终本地状态。

不建议在普通业务日志中打印完整消息内容或摘要全文。摘要内容和消息内容可能包含敏感信息，应使用受控日志或数据库审计策略。

建议监控：

- Agent RPC 成功率、超时率、不可用率；
- `WAITING_USER_DECISION` 数量；
- 各状态任务数量；
- 摘要处理耗时；
- 摘要游标滞后数量；
- 同一 run 的重复请求和并发冲突数；
- 输入消息数量超限次数。

## 16. 依赖注入与落地文件建议

推荐按以下顺序实施：

### 第一步：协议与依赖

- 增加 `proto/room_agent/v1/summary.proto`；
- 确认 `go_package`；
- 配置 protobuf/gRPC 代码生成；
- 增加 `google.golang.org/grpc` 依赖；
- 生成 `summary.pb.go` 和 `summary_grpc.pb.go`。

### 第二步：配置

- 在 `configs/entry.go` 增加 Agent 配置结构；
- 在配置文件中增加 endpoint、超时、TLS、认证和消息限制；
- 增加配置校验。

### 第三步：领域端口和 gRPC adapter

建议新增：

```text
backend/internal/application/ports/agent/room_summary_agent.go
backend/internal/application/summary/room_summary.go
backend/internal/infrastructure/agent/grpc/client.go
backend/internal/infrastructure/agent/grpc/mapper.go
backend/internal/infrastructure/summary/worker.go
```

实际命名以项目现有目录约定为准。应用端口不应暴露 protobuf 类型。

### 第四步：摘要持久化

- 添加数据库 migration；
- 增加 `room_summary_runs`；
- 增加 `room_summary_requests`；
- 增加 `room_summary_cursors`；
- 增加 `room_summary_events`；
- 实现 repository port 和 MySQL repository；
- 增加事务更新和行锁逻辑。

### 第五步：消息查询

- 在消息 repository port 增加 `ListMessagesForSummary`；
- 实现 `(afterSeq, untilSeq]` 查询；
- 保证按 seq 升序；
- 支持消息数限制；
- 明确撤回、媒体、系统消息等类型的传递规则。

### 第六步：应用服务

- 实现 START；
- 实现 RETRY；
- 实现 CANCEL；
- 实现状态机校验；
- 实现房间成员权限校验；
- 实现成功后游标推进；
- 实现并发控制。

### 第七步：前端 SSE 对接

- 增加经过鉴权的摘要 SSE 订阅流；
- SSE 只推送摘要事件，不直接暴露 Agent gRPC 响应；
- 事件包含 `id`、`event` 和 JSON `data`，并使用持久化 event ID；
- 支持 `Last-Event-ID` 断线补发；
- 事件过期后发送任务当前状态快照；
- Worker 完成后通过本地 SSE Hub 推送，跨实例使用 Redis 通知；
- 前端增加 `EventSource` 客户端，处理 RUNNING、SUCCEEDED、WAITING_USER_DECISION、FAILED；
- 前端仍通过既有的业务控制方式触发摘要操作，START/RETRY/CANCEL 最终统一进入后端任务队列；
- 保留现有 WebSocket 处理 IM 消息和已读回执，不把摘要事件塞入聊天 protobuf。

SSE 的鉴权方式需要结合现有登录态确认。浏览器原生 `EventSource` 不方便自定义 Authorization header，优先使用安全 Cookie；如果必须使用 token，应避免把长期有效 token 直接放在 URL 中。

### 第八步：服务启动和关闭

- 在 `main.go` 创建 gRPC connection；
- 创建 Agent adapter；
- 创建摘要 application service；
- 创建并启动 SummaryWorker；
- 注入内部调用方；
- 服务退出时关闭 connection。

Agent 不新增对外 HTTP/WS 接口；前端通过摘要 SSE 订阅流接收结果，摘要用例既可以由其他后端内部业务或任务调度器创建，也可以由已有控制入口触发，后续统一进入持久化任务队列。

## 17. 测试方案

### 17.1 gRPC 协议测试

- 校验 START、RETRY、CANCEL 的 protobuf 字段；
- 校验 `ResumeSummaryState` 只携带 request ID、run ID 和 scope；
- 校验 `request_id` 和 `summary_run_id` 传递；
- 校验 RETRY/CANCEL 不携带消息；
- 校验 Timestamp 的毫秒转换；
- 校验 Agent 返回结果 ID 与请求 ID 不一致时拒绝落库。

### 17.2 应用服务测试

- 非房间成员不能发起摘要；
- 不同用户不能操作其他用户的 run；
- START 使用正确的 `(afterSeq, untilSeq]` 范围；
- 成功时推进游标；
- FAILED、WAITING_USER_DECISION、CANCEL 不推进游标；
- Resume 不创建新的 run，不触发用户 retry 语义；
- RETRY 使用原 run ID；
- 新一轮 START 使用新 run ID；
- 同一 run 并发 RETRY/CANCEL 时只有一个请求可以执行。

### 17.3 gRPC mock server 测试

至少覆盖：

- SUCCEEDED；
- WAITING_USER_DECISION；
- FAILED；
- `INVALID_ARGUMENT`；
- `PERMISSION_DENIED`；
- `DEADLINE_EXCEEDED`；
- Agent 返回错误 ID；
- Checkpoint 不存在；
- Resume 返回 RUNNING、SUCCEEDED、WAITING_USER_DECISION、FAILED；
- 网络中断后的 UNKNOWN 状态处理。

### 17.4 数据库测试

- migration 可以在空库执行；
- migration 可以重复检查而不会破坏已有数据；
- 游标和任务状态更新具备事务原子性；
- 多实例并发下不会产生同一个活动 run 的两个 RPC。

## 18. 待确认事项

开始编码前需要和 Agent 服务提供方确认：

1. `summary.proto` 的真实文件内容和 `go_package`；
2. Agent gRPC 地址、TLS 证书、认证 metadata；
3. `ResumeSummaryState` 返回 Checkpoint 不存在时，本地任务应该进入失败、人工介入还是补偿重建；
4. `ResumeSummaryState` 对 START、RETRY、CANCEL 未知结果的恢复边界；
5. `status` 的正常值到底使用 `"normal"`、空字符串还是其他协议约定；
6. 没有增量消息时是否由后端仍然发送 START；协议允许传空 `room_messages`，Agent 将返回空摘要并标记为 `SUCCEEDED`；
7. 摘要是否按用户隔离；当前方案按 `(room_id, user_id)` 保存游标；
8. 单次摘要允许的最大消息数量和最大请求大小；
9. 是否允许同一用户同一房间存在多个未完成摘要任务；
10. Agent 返回的 `SummaryOutput` 和 `ErrorInfo` 的完整字段定义。

## 19. 验收标准

满足以下条件后，才认为后端 gRPC 对接完成：

- 后端能够通过一个长生命周期 gRPC client 调用 Agent；
- START、RETRY、CANCEL 的 ID 和 scope 规则全部符合协议；
- 摘要消息只来自经过权限校验的房间；
- 消息输入按照固定 seq 范围生成，并且可重建；
- 摘要成功后结果和游标原子更新；
- Agent 等待人工决策时能够持久化 `RetryRequired`；
- CANCEL 后任务为 FAILED，游标不推进；
- gRPC 错误不会被误当成成功摘要；
- 同一个 `summary_run_id` 不会并发请求；
- 网络超时不会在未确认幂等语义前盲目创建重复 START；
- 具备 mock gRPC server、状态机、映射、权限和并发测试；
- Agent 只接受后端 gRPC 调用，前端不能直连 Agent；
- 前端通过 SSE 接收摘要状态和结果；
- SSE 支持 `Last-Event-ID` 断线补发或任务快照恢复；
- 摘要不会阻塞现有 WebSocket 的 IM 消息处理；
- 现有 WebSocket 继续负责聊天实时通信，摘要不与聊天帧耦合。
