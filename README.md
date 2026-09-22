# SYCHAT

一个面向群聊场景的实时房间协作平台。

## 项目背景

基于 Go + Vue 3 的实时房间协作平台，包含账号、好友、房间、聊天、视频、一起看、回放等模块，支持 WebSocket 实时同步、视频上传与分片上传、房间内共享播放、历史消息与历史视频回放。

## 技术栈

- Go 1.25.2
- Gin
- GORM
- MySQL 8
- Redis 7
- Kafka 3.9
- MinIO
- gRPC
- Server-Sent Events (SSE)
- Vue 3
- Vite

---

## WebSocket 实时通道

```
路径: GET /api/v1/ws  (JWT 鉴权)
连接: 每个用户一个 WebSocket 连接，通过 sessionId 标识
心跳: 服务端定期 Ping，客户端回复 Pong
批量能力: 连接参数带 `batch=1` 时，服务端启用批量帧；不带参数时保持单帧兼容
```

### WebSocket Op 一览

| Op | 方向 | 说明 | Payload Proto |
|----|------|------|---------------|
| `msg` | 前端→后端 | 发送消息 | `MessageReq` |
| `msg` | 后端→接收方 | 收到新消息 | `MessageEvent` |
| `msg_ack` | 后端→发送方 | 服务端确认消息收到 | `MessageAck` |
| `msg_read_ack` | 前端→后端 | 已读某条消息 | `MessageReadAckReq` |
| `msg_read_notify` | 后端→发送方 | 你的消息被读了 | `MessageReadAckEvent` |
| `conversation_sync_seq` | (Kafka 内部) | 同步会话最大 seq | `ConversationSyncSeqEvent` |
| `watch_video_control` | 前端→后端 | 视频播放控制（播放/暂停/seek） | `WatchVideoControl` |
| `watch_video_sync` | 后端→房间成员 | 视频播放状态同步 | `WatchVideoState` |

### WebSocket 帧格式

```protobuf
message WsFrame {
  string op = 1;    // 操作类型
  bytes data = 2;   // payload，按 op 对应不同的 proto message 编码
}

message WsBatch {
  repeated WsFrame frames = 1;
}
```

批量模式下外层 `WsFrame.op` 为 `msg_batch`，`data` 解码为 `WsBatch`；客户端按 `frames` 原顺序分发每个内部 `op`。

---

## 数据流

### 1. 发送消息 (msg)

```
[发送方前端]                    [后端]                         [Kafka]                    [Consumer]              [接收方前端]
     │                            │                              │                           │                        │
     │── ws op="msg" ──→          │                              │                           │                        │
     │   MessageReq {              │                              │                           │                        │
     │     clientMsgId             │                              │                           │                        │
     │     recvId (对方/房间ID)     │                              │                           │                        │
     │     convType                │                              │                           │                        │
     │     cType                   │                              │                           │                        │
     │     content                 │                              │                           │                        │
     │     videoTime               │                              │                           │                        │
     │   }                         │                              │                           │                        │
     │                             │── HandleMessage               │                           │                        │
     │                             │   │ 校验成员/好友关系          │                           │                        │
     │                             │   │ 生成 messageId (Snowflake) │                           │                        │
     │                             │   │ Redis INCR → seq          │                           │                        │
     │                             │   │ DB: Save message          │                           │                        │
     │                             │   │ DB: Upsert conversation   │                           │                        │
     │                             │   │ DB: UpdateReadSeq/SyncSeq │                           │                        │
     │                             │   └─ SendMessage()            │                           │                        │
     │                             │      Envelope {                │                           │                        │
     │                             │        From: senderId         │                           │                        │
     │                             │        To:   recvId           │                           │                        │
     │                             │        Payload: MessageEvent  │                           │                        │
     │                             │      }                         │                           │                        │
     │                             │── topic="msg", key=convId ──→ │                           │                        │
     │   ←── ws op="msg_ack" ──    │                              │                           │                        │
     │   MessageAck {               │                              │                           │                        │
     │     clientMsgId, messageId,  │                              │                           │                        │
     │     status="sent"            │                              │                           │                        │
     │   }                         │                              │                           │                        │
     │                             │                              │── ConsumeClaim ──→       │                        │
     │                             │                              │   handleMessage()        │                        │
     │                             │                              │   ├─ PrivateChat:         │                        │
     │                             │                              │   │  SendToClient(        │── ws op="msg" ──→      │
     │                             │                              │   │    "msg", recvId,      │   MessageEvent          │
     │                             │                              │   │    MessageEvent)       │                        │
     │                             │                              │   │  UpdateSyncSeq(recvId) │                        │
     │                             │                              │   │                        │                        │
     │                             │                              │   └─ RoomChat:            │                        │
     │                             │                              │      并行 dispatch 给      │── ws op="msg" ──→      │
     │                             │                              │      所有房间成员          │   (每个成员)             │
     │                             │                              │      BatchUpdateSyncSeq    │                        │
```

**关键字段**：

| 字段 | 说明 |
|------|------|
| `Envelope.From` | 发送方 userId，consumer 不用于路由 |
| `Envelope.To` | 私聊时为接收方 userId，群聊时 consumer 自取成员列表 |
| `Envelope.Payload` | `MessageEvent` JSON，含 messageId/seq/content/senderId 等 |
| Kafka key | conversationId，保证同一会话消息有序 |

---

### 2. 已读回执 (msg_read_ack → msg_read_notify)

```
[读者前端]                      [后端]                          [Kafka]                    [Consumer]              [发送方前端]
     │                            │                               │                           │                        │
     │── ws op="msg_read_ack" ─→  │                               │                           │                        │
     │   MessageReadAckReq {       │                               │                           │                        │
     │     conversationId          │                               │                           │                        │
     │     lastReadSeq             │                               │                           │                        │
     │     senderId ← 前端已知      │                               │                           │                        │
     │   }                         │                               │                           │                        │
     │                             │── HandleMessageReadAck         │                           │                        │
     │                             │   │ 校验 senderId 非空且非读者  │                           │                        │
     │                             │   │ DB: GetUserConversation    │                           │                        │
     │                             │   │ 若 lastReadSeq ≤ 已记录值  │                           │                        │
     │                             │   │   → 直接 return (幂等)     │                           │                        │
     │                             │   │ DB: UpdateReadSeq          │                           │                        │
     │                             │   │ DB: GetByID (取 ConvType)  │                           │                        │
     │                             │   └─ SendMessageReadAck()      │                           │                        │
     │                             │      事件中携带:                │                           │                        │
     │                             │        - userId (读者)          │                           │                        │
     │                             │        - convType (前端展示用)  │                           │                        │
     │                             │        - senderId (路由目标)    │                           │                        │
     │                             │── topic="msg_read_ack" ────→  │                           │                        │
     │                             │                               │── ConsumeClaim ──→       │                        │
     │                             │                               │   handleMessageReadAck() │                        │
     │                             │                               │   SendToClient(          │── ws op="msg_read_notify" ──→
     │                             │                               │     "msg_read_notify",    │   MessageReadAckEvent {
     │                             │                               │     senderId,             │     userId (读者)
     │                             │                               │     MessageReadAckEvent)  │     conversationId
     │                             │                               │                           │     lastReadSeq
     │                             │                               │                           │     convType
     │                             │                               │                           │     senderId
     │                             │                               │                           │   }
```

**设计要点**：
- 前端展示消息时已知发送方，直接在 ack 中回传 `senderId`，**后端无需查询消息表**
- 私聊和群聊逻辑完全一致——都是一条消息对应一个发送方
- `msg_read_ack` 是读者的确认，`msg_read_notify` 是给发送方的通知，两个不同 op
- 幂等：重复 ack（seq 未增长）直接返回 nil

---

### 3. 离线消息与会话同步

```
[前端]                              [后端]                            [Kafka]                 [Consumer]
  │                                   │                                 │                        │
  │── GET /messages/offline ──→       │                                 │                        │
  │                                   │── GetOfflineMessages              │                        │
  │                                   │   │ ListByUser(userId)            │                        │
  │                                   │   │ → 所有 UserConversation       │                        │
  │                                   │   │ ListByIDs(convIds)            │                        │
  │                                   │   │ → 所有 Conversation           │                        │
  │                                   │   │ 计算 unreadMap                │                        │
  │                                   │   │   = latestSeq - readSeq       │                        │
  │                                   │   │ GetLatestMessagesByConvIDs    │                        │
  │                                   │   │ → 每个会话最新一条消息         │                        │
  │                                   │   │                               │                        │
  │                                   │   │ 对比 latestSeq vs syncSeq     │                        │
  │                                   │   │ 若有差距 → 异步发 sync seq     │                        │
  │                                   │   └─ SendConversationSyncSeq() ──→│── ConsumeClaim ──→    │
  │                                   │      topic="conversation_sync_seq"│   handleConvSyncSeq() │
  │                                   │                                   │   BatchUpdateSyncSeq  │
  │   ←── { messages[], unread {} } ──│                                   │                        │
```

**ConversationSyncSeq 的用途**：当离线消息加载后，如果用户本地缓存的 `syncSeq` 落后于会话实际 `latestSeq`，通过 Kafka 批量更新 `UserConversation.LatestSyncSeq`，保证下次获取离线消息时不再重复拉到已看过的。

---

### 4. 视频一起看 (watch_video_control → watch_video_sync)

```
[操作者前端]                        [后端]                              [房间其他成员]
     │                                │                                      │
     │── ws op="watch_video_control" → │                                      │
     │   WatchVideoControl {           │── CheckRoomMember                     │
     │     roomId                      │── 若 action="get_state"               │
     │     action (play/pause/seek)    │     → 直接返回当前状态给操作者         │
     │     videoId                     │── UpsertWatchVideoState (内存)         │
     │     positionMs                  │── GetRoomMemberIDs                    │
     │     playbackRate                │── SendWatchVideoStateToUsers ────────→│
     │     ...                         │    op="watch_video_sync"              │
     │                                 │    (广播给所有房间成员含操作者)         │
```

**播放状态**：目前以内存态存储（Gateway 持有），进程重启会丢失（待完善项）。

**释放机制**：用户断开 WebSocket 时，`ReleaseWatchVideoStatesByUser` 遍历该用户持有的共享状态，通知房间成员状态变更。

---

### 5. WebSocket 连接生命周期

```
[前端]                              [后端]
  │                                   │
  │── GET /ws (JWT token) ──→         │
  │                                   │── 验证 JWT → userId
  │                                   │── 创建 sessionId (UUID)
  │                                   │── NewClient(ctx, conn, userId, sessionId)
  │                                   │── gateway.AddClient(client)
  │                                   │── go readLoop(client)
  │                                   │── go writeLoop(client)
  │                                   │
  │                                   │   readLoop:
  │                                   │   │ 设置 ReadDeadline + PongHandler
  │                                   │   │ for { client.Read() → dispatcher.Dispatch(op, data) }
  │                                   │   │ 异常退出时:
  │                                   │   │   client.Close()
  │                                   │   │   gateway.RemoveClient()
  │                                   │   │   ReleaseWatchVideoStatesByUser()
  │                                   │   │   → 广播释放后的播放状态给房间成员
  │                                   │
  │                                   │   writeLoop:
  │                                   │   │ 定时 Ping
  │                                   │   │ for { select case msg ← client.send:
  │                                   │   │          proto.Marshal(WsFrame{Op, Data})
  │                                   │   │          WriteMessage() }
  │                                   │
  │   ←── Ping ──                     │
  │   ── Pong ──→                     │  (重置 idle 时间戳)
```

### 5.1 WebSocket 断线重连与数据补齐

```
[前端]                                                     [后端]
  │                                                          │
  │── onclose ──→ scheduleWsReconnect                         │
  │   │  最多重试 5 次，指数退避                                │
  │   │  wsReconnecting = true                                │
  │   │                                                       │
  │   └── 重连成功 ──→ onopen                                  │
  │       │  wasReconnecting = true                           │
  │       │  wsConnected = true                               │
  │       │  onWsReconnect() ──→ loadOffline()                 │
  │       │                       │                            │
  │       │                       │── GET /messages/offline ──→│
  │       │                       │   │ 返回所有会话 + 未读数   │
  │       │                       │   │ 返回每个会话最新消息     │
  │       │                       │   │ 异步触发 sync seq      │
  │       │                       │                            │
  │       │                       │←── conversations[] ────────│
  │       │                       │   更新侧边栏列表 + 未读计数  │
  │       │                       │   当前会话从 localStorage   │
  │       │                       │   恢复消息（如有缓存）       │
  │       │                       │                            │
  │       │  若重试耗尽 → wsReconnectFailed = true              │
  │       │  用户可点击状态点手动重连                             │
```

**关键点**：
- `onopen` 只负责设连接状态 + 触发回调，不自行拉数据
- `onWsReconnect` 回调由 `useImClient` 注入，调用 `conversation.loadOffline()` 补齐断线期间消息
- 首次连接**不**触发（`wasReconnecting` 为 false），避免与 `submitAuth` 的 `loadOffline` 重复
- 重连失败后用户可手动点击状态指示器调用 `retryWsConnection` 重新连接

---

## 摘要 Agent

摘要功能采用“HTTP 初始化任务 + gRPC 调用 Agent + SSE 获取结果”的组合方式：

- HTTP 负责鉴权、房间成员校验、任务状态管理和 SSE 连接
- gRPC 负责调用独立的 Room Summary Agent
- SSE 负责把已保存的摘要结果或 Agent 处理结果推送给前端
- MySQL 保存摘要任务的当前状态和最新响应，Redis 保存未读消息快照以及同一用户同一房间的任务绑定

### 摘要接口

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/rooms/:roomId/summaries` | JWT | 初始化摘要任务，返回 `summaryRunId` |
| GET | `/rooms/:roomId/summaries/:summaryRunId/events` | JWT | 建立 SSE，接收摘要结果 |
| POST | `/rooms/:roomId/summaries/:summaryRunId/retry` | JWT | 对等待人工决策的任务执行重试 |
| POST | `/rooms/:roomId/summaries/:summaryRunId/cancel` | JWT | 取消等待人工决策的任务 |

### 摘要任务链路

```text
[前端]
   │
   │ POST /rooms/:roomId/summaries
   ▼
[HTTP Handler]
   │ 1. JWT 鉴权、校验房间成员
   │ 2. 读取用户未读范围（fromSeq ~ toSeq）
   │ 3. 查询消息，过滤已撤回消息
   ▼
[Redis scope 租约]
   │ key = userId + roomId，TTL 2 分钟，带 lockToken
   │ 已存在任务时直接复用原 summaryRunId
   ▼
[MySQL summary_runs]
   │ 创建 RUNNING 任务，保存消息序号范围
   ▼
[后台 goroutine]
   │ RunSummary(START + room_messages)
   ▼
[Agent gRPC]
   │ 返回 SUCCEEDED / WAITING_USER_DECISION / FAILED
   ▼
[MySQL]
   │ 保存最新 requestId、状态和 response_payload
   ▼
[SSE subscribers]
   │ 向同一个 summaryRunId 下的 SSE 连接广播结果
   ▼
[前端]
```

初始化接口返回 `202 Accepted` 和 `summaryRunId`。当前实现会在初始化成功后立即异步预热调用 Agent，前端随后建立 SSE；因此 SSE 建立时如果 Agent 已经完成，会先从 `summary_runs.response_payload` 回放最新结果，如果仍在执行，则等待内存订阅收到结果。

### SSE 行为

- SSE 建立时会再次校验 `summaryRunId` 的房间和用户归属，防止越权访问。
- 连接建立后先发送 `ready` 事件，随后每 15 秒发送注释心跳 `: ping`。
- Agent 完成后，后端先落 MySQL，再向当前任务的所有 SSE 订阅者广播。
- SSE 断开只会移除当前订阅，不会取消 Agent 任务；客户端可以使用同一个 `summaryRunId` 重新连接并读取已保存结果。
- 同一个任务允许多个设备建立 SSE，但它们消费的是同一条任务结果，不会各自生成摘要。

### 状态和用户决策

```text
RUNNING
  ├── SUCCEEDED
  ├── FAILED
  └── WAITING_USER_DECISION
          ├── RETRY  → RUNNING → Agent RunSummary(RETRY)
          └── CANCEL → FAILED  → Agent RunSummary(CANCEL)
```

`RETRY` 和 `CANCEL` 使用原来的 `summaryRunId`，不重新提交房间消息。后端在 MySQL 事务中使用 `SELECT ... FOR UPDATE` 锁定任务，并且只允许 `WAITING_USER_DECISION` 状态执行操作，提交状态变更后再异步调用 Agent。

### 持久化与幂等

`summary_runs` 当前保存：

- `summary_run_id`、`room_id`、`user_id`
- 任务状态和摘要使用的 `from_seq`、`to_seq`
- 最近一次 `request_id`
- Agent 返回的完整 `response_payload`
- 创建、更新时间和完成时间

Redis 主要保存两类数据：

- 未读摘要快照：`summary:active:room:unread:{userId}:{roomId}`，固定本次摘要的消息范围
- 任务 scope 租约：`summary:scope:run:{userId}:{roomId}`，保证同一个用户在同一个房间只绑定一个活动 `summaryRunId`

scope 租约由初始化请求创建，Agent 执行期间使用原 `lockToken` 按 TTL 的一半周期续期；初始化落库失败时立即释放，Agent 返回成功或失败后停止续期并等待 TTL 过期。Redis 丢失时，后端还会查询 MySQL 中 `RUNNING` 或 `WAITING_USER_DECISION` 的活动任务作为兜底。

### 当前 MVP 边界

当前实现保存的是 `summary_runs` 的最新响应，还没有独立的 `summary_events` 事件表和 `event_seq` 历史，因此 SSE 重连采用“回放最新结果”的方式，不是按 `Last-Event-ID` 补发多条历史事件。gRPC 客户端已具备 `ResumeSummaryState` 映射能力，后续可以在服务重启恢复、事件历史和断点续传场景中接入。

相关代码：

- 应用编排：[summary_application.go](backend/internal/application/agent/summary_application.go)
- HTTP/SSE：[handle.go](backend/internal/transport/http/agent/handle.go)、[router.go](backend/internal/transport/http/agent/router.go)
- gRPC 客户端：[client.go](backend/internal/infrastructure/agent/grpc/client.go)
- 摘要任务仓储：[summary.go](backend/internal/infrastructure/persistence/mysql/repository/summary/summary.go)
- Redis 快照与 scope 租约：[summary.go](backend/internal/infrastructure/persistence/redis/cache/summary/summary.go)

---

## Kafka 架构

### Topic 一览

| Kafka Topic | 生产者 | 消费者 | 说明 |
|-------------|--------|--------|------|
| `msg` | TaskManager.SendMessage | GroupHandler.handleMessage | 新消息事件 |
| `msg_read_ack` | TaskManager.SendMessageReadAck | GroupHandler.handleMessageReadAck | 已读回执，消费后转 `msg_read_notify` 分发给发送方 |
| `conversation_sync_seq` | TaskManager.SendConversationSyncSeq | GroupHandler.handleConversationSyncSeq | 离线加载后批量同步 seq |

### Envelope 模式

所有 Kafka 消息都包装为 Envelope：

```go
type Envelope struct {
    From    string  // 消息来源 userId（部分场景留空）
    To      string  // 路由目标 userId / conversationId
    Payload []byte  // 具体事件 JSON
}
```

Consumer 根据 topic 类型 decode `Payload` 为对应事件，再按业务逻辑 dispatch 到目标 WebSocket 客户端。

### Consumer dispatch 策略

| Topic | 私聊 | 群聊 |
|-------|------|------|
| `msg` | 发给 `envelope.To`（对方） | 从缓存/DB 取成员列表，并行 dispatch 给所有人（≤100 人时） |
| `msg_read_ack` | 从事件中取 `senderId`，dispatch `msg_read_notify` | 同私聊（一条消息一个发送方） |
| `conversation_sync_seq` | 不 dispatch，仅批量更新 DB 的 `LatestSyncSeq` | 同 |

---

## 协议结构

### 请求 (前端→后端)

```protobuf
message MessageReq {
  string client_msg_id = 1;
  string recv_id = 2;       // 私聊: 对方userId, 群聊: roomId
  int32 conv_type = 3;      // 1=私聊, 2=群聊
  int32 c_type = 4;         // 消息内容类型
  string content = 5;
  int64 video_time = 6;
  bool has_video_time = 7;
}

message MessageReadAckReq {
  string conversation_id = 1;
  int64 last_read_seq = 2;
  string sender_id = 3;     // 被读消息的发送方 (前端已知)
}

message WatchVideoControl {
  string room_id = 1;
  string action = 2;        // play/pause/seek/get_state
  string video_id = 3;
  int64 position_ms = 5;
  double playback_rate = 8;
  // ...
}
```

### 响应 (后端→前端)

```protobuf
message MessageAck {
  string client_msg_id = 1;
  string message_id = 2;
  string status = 3;        // "sent" | "failed"
  string extra = 4;
}

message MessageEvent {
  string message_id = 1;
  string conversation_id = 2;
  string sender_id = 3;
  string recv_id = 4;
  int64 seq = 5;
  int32 conv_type = 6;
  string content = 8;
  int64 send_time = 9;
  string sender_username = 10;
  // ...
}

message MessageReadAckEvent {
  string user_id = 1;        // 读者
  string conversation_id = 2;
  int64 last_read_seq = 3;
  int32 conv_type = 4;
  string sender_id = 5;      // 通知目标 (消息发送方)
}

message WatchVideoState { /* ... */ }
```

### 内部事件 (Go 侧，经 Kafka 传输)

```go
type Event struct {
    Type EventType       `json:"type"`   // msg / msg_read_ack / conversation_sync_seq
    Data json.RawMessage `json:"data"`   // 具体事件 JSON
}

type ConversationSyncSeqEvent struct {
    Items []ConversationSyncSeqItem `json:"items"`
}
```

---

## 功能模块与接口

### 账户模块

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/users/register` | 否 | 手机号注册 |
| POST | `/users/login` | 否 | 手机号登录 |
| POST | `/users/logout` | JWT | 退出登录 |
| GET | `/users/:userId` | JWT | 获取用户信息 |
| GET | `/users/resolve` | JWT | 通过用户名或手机号解析用户 |

### 好友模块

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/friends` | JWT | 获取好友列表 |
| POST | `/friend-requests` | JWT | 发送好友申请 |
| GET | `/friend-requests` | JWT | 获取好友申请列表 |
| POST | `/friend-requests/actions` | JWT | 同意或拒绝好友申请 |

### 聊天模块

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/messages/history` | JWT | 获取会话历史消息 |
| GET | `/messages/offline` | JWT | 获取离线消息与会话摘要 |

### 房间模块

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/rooms` | JWT | 创建房间 |
| GET | `/rooms/:roomId/invite-code` | JWT | 获取或刷新房间邀请码 |
| POST | `/rooms/join` | JWT | 通过邀请码加入房间 |

### 摘要模块

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/rooms/:roomId/summaries` | JWT | 初始化并预热摘要任务 |
| GET | `/rooms/:roomId/summaries/:summaryRunId/events` | JWT | 建立摘要 SSE 连接 |
| POST | `/rooms/:roomId/summaries/:summaryRunId/retry` | JWT | 重试摘要任务 |
| POST | `/rooms/:roomId/summaries/:summaryRunId/cancel` | JWT | 取消摘要任务 |

### 视频模块

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/files` | JWT | 上传视频文件 |
| POST | `/files/multipart/init` | JWT | 初始化分片上传 |
| PUT | `/files/multipart/:uploadId/parts/:partNumber` | JWT | 上传视频分片 |
| POST | `/files/multipart/:uploadId/complete` | JWT | 完成分片上传 |
| GET | `/files/:fileId` | JWT | 获取视频文件信息或播放地址 |

### 一起看模块

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/ws` | JWT | WebSocket 实时同步播放状态 |

### 回放模块

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/messages/videos` | JWT | 获取房间历史视频列表 |
| GET | `/messages/danmaku` | JWT | 获取房间视频弹幕回放 |

### 实时通道

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/ws` | JWT | 实时消息收发、已读确认、播放状态同步 |

---

## 待完善项

- 播放状态目前主要依赖内存态，进程重启后需要补全房间播放状态恢复
- 控制权模型目前是"当前共享者独占"，后续可扩展为申请接管、主持人转移和超时释放
- 房间侧还可以补充房间列表、房间详情和成员在线状态展示
- 可以继续增强共享播放的可观测性，例如日志审计、限流和异常兜底
- 回放能力目前偏功能型，后续可继续做成更完整的会话回放体系

## 接口文件

- `docs/apipost.collection.json`：Apipost 可直接导入的接口集合文件
