# 后端功能闭环审查

审查范围：`backend/` 当前代码。本文只评估后端；前端暂不展开，仅记录后端能力尚未被前端接入的部分。

结论：当前不建议直接上线生产。单聊、小群消息的核心链路已经具备，文件上传和大群提醒也有主要骨架，但可靠性和运维闭环仍缺少关键部分。

## 功能状态

| 功能 | 当前已实现 | 未闭环/风险 | 等级 |
|---|---|---|---|
| 登录、JWT 鉴权 | 注册、登录、刷新/退出接口，HTTP/WS 鉴权中间件 | 退出逻辑仍有 TODO，需确认 token 黑名单/撤销策略；JWT 失效只能等待过期 | 中 |
| 好友关系 | 好友申请创建、接受/拒绝、好友关系事务、重复申请唯一约束 | 拒绝不产生提醒是当前设计；需补充重复申请、双向申请、已是好友等并发测试 | 中 |
| 好友申请提醒 | 申请事务写入 `outboxes`，Outbox -> Kafka -> `friend_request_created` -> 在线 WS | 只通知在线用户；无离线红点/未读持久化，Kafka 消费幂等未接入 | 高 |
| 单聊消息 | WS 接收，鉴权/限流，事务写入 `messages`、会话最新 seq、发送者已读 seq、媒体子表和 Outbox | Kafka 投递成功后进程崩溃、尚未标记 sent 时会重复消费；Inbox 未接入导致副作用幂等缺失 | 高 |
| 小群消息 | 校验房间成员；小群读取成员并逐个在线推送；会话/消息持久化 | 推送失败时 Kafka 重试可能造成已成功用户重复收取；没有面向客户端的消息去重协议说明 | 高 |
| 大群消息 | `MemberCount > 500` 时跳过全员完整消息 fanout；按会话合并 seq，向在线成员推送轻量通知 | 轻量通知丢失后依赖客户端主动 `/messages/sync`；客户端需要保存每个会话的 `lastReadSeq` | 高 |
| 消息历史 | `/messages/history` 校验会话权限、游标分页、批量加载媒体扩展和文件元信息 | 预签名 URL 生成/缓存失败会直接影响历史返回；需补充大量消息、过期附件、文件不存在的测试 | 中 |
| 离线同步 | `/messages/sync` 按 `conversationId + afterSeq` 一次返回离线增量，不分页，服务端以会话当前最新序号为边界 | 没有全局同步游标/会话变更同步；好友申请、房间成员变化没有统一离线同步接口 | 高 |
| 已读回执 | WS `msg_read_ack`；单调推进 `last_read_seq`；查询区间内不同发送者；写入 Outbox/MQ；在线通知发送者 | 只有最后水位，没有逐条已读记录；通知只覆盖在线用户；重复/乱序消费仍依赖 Inbox，但当前未接入 | 中 |
| 文件直传 | 小文件后端接收并上传 MinIO；校验前端 hash；文件入库；数据库唯一键兜底；缓存回填；消息发送时校验 `fileId` | 文件入库失败后的 MinIO 清理是请求内补偿，清理失败只日志记录，没有异步重试/孤儿扫描 | 高 |
| 文件分片上传 | `init -> file_uploads(uploading) -> Redis 元数据 -> 批量预签名 -> MinIO 分片 -> complete`；完成时事务写 `files` 并标记 `file_uploads.completed`；合并锁有续期；已接入过期清理 Worker、MinIO Abort、Redis 清理和失败重试 | `cleanup_failed` 任务需要告警和人工处理；仍需验证真实 MinIO 超时/中断场景 | 高 |
| 文件消息附件 | 消息事务同时写媒体子表和 `message_attachments`；历史按 attachment 查 file；访问 URL 校验会话成员 | 文件上传成功但消息发送失败会留下未引用文件；没有基于 `files` 状态/引用关系的回收任务 | 高 |
| 资源访问 | `GET /files/attachments/:attachmentId/access-url` 校验用户是否属于会话，再从 file 元信息生成预签名 URL | 访问失败、过期、对象已删除的错误路径需监控；附件过期后的清理策略未实现 | 中 |
| 会话列表 | `/conversations` 返回私聊/房间、最新 seq、lastReadSeq、unread、成员数等 | 会话创建、好友接受、入群等事件的离线补偿需统一验证；缓存与 MySQL 不一致主要依靠旁路回源/删除 | 中 |
| 房间成员状态 | MySQL version、Redis 成员缓存、成员变更 Outbox/MQ、在线通知 | 缓存删除失败后的重试/CDC/Outbox 兜底不完整；需验证消费乱序时 version 是否真正拒绝旧状态 | 高 |
| WebSocket 会话 | 心跳、批量发送、单独 accumulator/send loop、自动重连基础设施 | 服务端 graceful shutdown、批次封口/发送失败补偿需压测；WS 断线期间消息完全依赖 HTTP sync | 中 |
| Outbox | 事务内写入；Worker claim、发布 Kafka、指数退避、dead 状态、DLQ publisher | `publish` 成功后 `MarkSent` 失败会重复发布；依赖消费者幂等，不能宣称端到端 exactly-once | 高 |
| Inbox | Kafka 消费入口已接入 `processing/completed/dead` 状态、锁超时抢占和 eventID 去重 | WS 推送等外部副作用与 Inbox 状态无法共用数据库事务，崩溃窗口内仍可能重复推送；需要客户端按 message/request/event ID 去重 | 高 |
| Kafka | Consumer group、消费错误重试、DLQ publisher 已有 | 启动连接失败直接 `Fatal`；本地/测试必须有 Kafka；DLQ 之后没有人工重放/告警闭环 | 高 |

## 关键未闭环

### 1. Inbox 已接入，但不是 exactly-once

当前 `inboxes` 已保存事件处理状态，Kafka 路由在执行业务 Handler 前抢占事件，成功后标记 `completed`，毒消息进入 `dead`；处理中的锁超时后可被重新抢占。

剩余影响：Handler 已经完成 WS 推送但进程在 `MarkCompleted` 前崩溃时，Kafka 会重试并再次推送。数据库 Inbox 无法和 WebSocket 发送组成同一个事务，因此必须在客户端和业务处理层使用 message/request/event ID 幂等。

上线前要求：补充重复投递、处理超时、进程崩溃恢复测试，并为 WS 客户端补齐事件去重。

### 2. `file_uploads` 清理已接入，但仍需运维闭环

当前已增加 `retry_count`、`next_retry_at`、`locked_at`、`last_error`，并由独立 Worker 扫描过期任务。Worker 使用事务抢占、MinIO `AbortMultipartUpload`、Redis 元数据清理和 active key 条件删除；失败按退避重试，超过上限标记 `cleanup_failed`。

剩余风险：`cleanup_failed` 记录不能静默堆积，需要监控、告警和人工/管理任务处理。还需要在真实 MinIO 环境验证 Abort 的幂等行为。

当前状态：分片上传的基础清理链路已闭环，但生产上线前仍需补充监控和故障演练。

### 3. 文件上传和消息发送是两个独立提交

当前后端先完成 MinIO/`files`，随后 WS 再创建消息和 `message_attachments`。消息事务失败时，文件已经存在但没有引用。

这不是数据正确性错误，但会产生孤儿文件。上线前至少要有：`files` 的引用状态或引用查询、未引用文件延迟回收任务、保留窗口和失败告警。不能在消息发送失败时立即无条件删除，因为客户端可能重试发送。

### 4. 大群实时通知不是离线消息投递

超过 500 人时，`message_delivery.go` 只合并每个会话的最新 seq，并调用 `DeliverToOnlineRoomMembers`。这是带宽优化，正确前提是客户端断线或漏通知后调用 `/messages/sync`。

当前后端没有统一的“用户上线后按所有会话水位补偿”接口，只提供按会话同步。因此大群功能可用，但不具备完整的离线通知/红点保证。若上线宣称可靠离线消息，需要补全用户级同步游标或会话水位拉取。

### 5. 缓存一致性主要依赖旁路策略，缺少最终兜底

会话、房间成员、用户资料等路径有缓存回源、singleflight 或失效处理，但没有统一的缓存失效失败重试队列。MySQL 更新成功、Redis 删除失败时，旧状态可能继续被读取到 TTL 到期。

当前可以接受为最终一致，但上线前应明确 TTL、版本比较和告警；成员状态必须在消费时拒绝旧 version，缓存失效失败需要 Outbox/重试任务兜底。

## 可以上线的边界

| 场景 | 结论 |
|---|---|
| 单机开发、功能演示 | 可以，前提是 MySQL、Redis、Kafka、MinIO 均可用 |
| 内测单聊/小群 | 修复 Inbox 接入和基础监控后可考虑 |
| 生产单聊/小群 | 当前不建议，重复消费和文件孤儿没有可靠闭环 |
| 生产大群 | 当前不建议，需明确 sync 补偿、在线通知丢失和容量保护 |
| 生产文件上传 | 清理链路已具备；完成监控、告警和 MinIO 故障演练后再上线 |

## 上线前优先级

1. 为 Inbox 和 Kafka 消费补充重复投递、锁超时、进程崩溃恢复测试。
2. 实现未引用 `files` 的延迟回收策略，覆盖“上传成功但消息发送失败”。
3. 补充 Kafka/Outbox 指标：pending、retry、dead、DLQ、消费延迟、重复事件数。
4. 明确并测试大群断线后的 `/messages/sync` 补偿和会话未读一致性。
5. 执行真实依赖环境下的集成测试：重复 Kafka 事件、进程崩溃、Redis 不可用、MinIO 合并超时、数据库事务失败。

## 尚未确认的前端接入项

本轮不审查前端实现，仅记录后端已提供但需要前端接入/验证的能力：

- 大群 `room_msg_notice` 收到后按会话和 seq 调用 `/messages/sync`。
- 断线重连成功后，对各会话补拉消息并恢复未读水位。
- 文件上传完成后携带 `fileId` 发送消息，并处理消息发送失败后的重试。
- 媒体消息使用 `attachmentId` 调用访问 URL，而不是持久化临时 URL。
- 收到消息并真正展示后发送 `msg_read_ack`，并处理 `msg_read_notify`。
- 好友申请提醒事件与离线好友申请列表/红点结合。
