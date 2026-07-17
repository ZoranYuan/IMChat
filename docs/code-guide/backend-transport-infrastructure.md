# 后端接口与基础设施

## 1. 启动与依赖组装

| 文件 | 作用 |
|---|---|
| `cmd/server/main.go` | 进程入口，初始化依赖、注册路由、启动 Worker，并优雅关闭 HTTP 服务 |
| `cmd/migrate/main.go` | 数据库迁移入口预留，目前没有正式迁移实现 |

`main.go` 是 Composition Root，即唯一集中了解“接口由哪个实现提供”的地方：

```go
// 基础设施实例
redisClient := redis.InitRedis(cfg.Database.Redis.DSN)
db := mysql.InitMysql(cfg.Database.MySQL.DSN)
txManager := persistence.NewGormTxManager(db)

// 端口实现
messageRepository := messagemysql.NewMessageRepository(db)
conversationCache := conversationredis.NewConversationCache(redisClient)
taskManager := messaging.NewTaskManager(kafkaProducer)

// 应用服务只接收接口，不在内部 new GORM/Redis/Kafka。
messageApplication := messageapp.NewMessageApplication(
    cfg, conversationCache, txManager, taskManager, /* repositories... */
)
```

为什么这样写：对象创建集中，业务对象只声明依赖。测试时可以传 Fake，运行时传 MySQL/Redis/Kafka 实现。

后台任务在这里启动：Kafka Consumer、Outbox Worker、WebSocket KeepAlive。路由也在这里统一挂到 `/api/v1`，开发数据路由只在 development 环境开放。

HTTP Server 配置超时并监听 `SIGINT/SIGTERM` 后调用 `Shutdown`，解决发布时直接杀进程导致正在执行的请求被截断的问题。

## 2. 共享协议

| 文件 | 作用 |
|---|---|
| `internal/shared/protocol/entry.go` | HTTP、应用、Kafka、WebSocket 共同使用的事件名、信封和事件结构 |

关键代码：

```go
type Envelope struct {
    From    string
    To      string
    Payload []byte
}

const (
    EventTypeMessage       = "msg"
    EventTypeMsgAck        = "msg_ack"
    EventReadMessageAck    = "msg_read_ack"
    EventReadMessageNotify = "msg_read_notify"
    EventWatchVideoCtrl    = "watch_video_control"
    EventWatchVideoSync    = "watch_video_sync"
)
```

为什么使用 Envelope：Kafka 基础设施只需要理解 From、To 和 Payload，不必与每种业务事件强耦合。事件负载变化时，Producer/Consumer 的通用框架不需要一起重写。

`MessageEvent` 同时携带公共消息字段和返回前端需要的媒体字段；MySQL 内部仍采用主表加子表，传输 DTO 不要求与数据库表一一对应。

## 3. HTTP Transport 文件索引

### 公共 HTTP 文件

| 文件 | 作用 |
|---|---|
| `transport/http/entry.go` | 将各模块 Router 挂到统一 API Group |
| `transport/http/response/entity.go` | 统一响应结构，包含业务码、消息和数据 |
| `transport/http/response/entry.go` | `Success`、`Error` 响应构造函数 |
| `transport/http/middleware/auth.go` | 从 Token 解析用户并写入 Gin Context |
| `transport/http/middleware/error_logger.go` | 记录 5xx、4xx、Context Error 和慢请求 |

鉴权中间件的核心模式：

```go
func (m *AuthMiddleware) JWTAuthMiddleware() gin.HandlerFunc {
  return func(c *gin.Context) {
    token := readBearerOrQueryToken(c)

    // JWT 签名/过期校验负责证明 Token 没有被篡改。
    claims, err := jwt.ValidateToken(token, m.config.JWT.Secret)
    if err != nil {
        c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(...))
        return
    }

    // Redis 登录态校验允许服务端撤销不再有效的 Access Token。
    userID, err := m.authCache.GetUserIDByAccessToken(c, token)
    if err != nil || userID != claims.UserID {
        c.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(...))
        return
    }

    // 后续 Handler 不再自行解析 JWT，并且不能相信请求体里的 userId。
    c.Set("userId", userID)
    c.Next()
  }
}
```

WebSocket 无法总是方便设置 Authorization Header，因此中间件同时需要兼容查询参数 Token。解析后只把可信的 `userId` 放入 Context，Handler 不能相信客户端请求体里的 senderId。

错误日志中间件在 `c.Next()` 之后记录状态码和耗时，因为只有业务 Handler 执行结束后才知道最终结果。慢请求单独标 WARN，便于开发阶段定位数据库或外部服务延迟。

### 用户 HTTP

| 文件 | 作用 |
|---|---|
| `transport/http/user/router.go` | 注册公开登录/注册路由和受保护用户路由 |
| `transport/http/user/handle.go` | 参数绑定、调用用户应用、错误到 HTTP 状态映射、设置 Refresh Cookie |
| `transport/http/user/dto.go` | 登录、注册请求和用户响应结构 |

关键代码：

```go
switch req.LoginType {
case int(uservo.PhoneType):
    user, err = app.LoginWithPhone(req.Phone, req.Password)
case int(uservo.WxType):
    // 类型预留不等于已经实现，明确返回不支持。
    c.JSON(http.StatusBadRequest, response.Error(...))
    return
default:
    c.JSON(http.StatusBadRequest, response.Error(...))
    return
}

switch {
case errors.Is(err, userapp.ErrUserNotFound):
    c.JSON(http.StatusNotFound, ...)
case errors.Is(err, userapp.ErrIncorrectPassword):
    c.JSON(http.StatusUnauthorized, ...)
}
```

为什么用 `errors.Is`：应用错误可能经过 `%w` 包装，直接 `err == target` 不能覆盖错误链。稳定的错误映射避免所有登录失败都变成 400 或 500。

Refresh Token 写入 HttpOnly Cookie，JavaScript 不能读取，降低 XSS 窃取风险；Access Token 仍由响应返回给前端用于请求和 WebSocket。

### 好友 HTTP

| 文件 | 作用 |
|---|---|
| `transport/http/friend/router.go` | 好友列表路由 |
| `transport/http/friend/handle.go` | 从 Context 获取当前用户并返回好友列表 |
| `transport/http/friend/dto.go` | 好友列表响应结构 |
| `transport/http/friend_request/router.go` | 创建、查询、处理好友申请的路由 |
| `transport/http/friend_request/handle.go` | 申请参数校验和 accept/refuse 分发 |
| `transport/http/friend_request/dto.go` | 好友申请请求与响应结构 |

Handler 不接受请求体中的当前用户 ID，而使用鉴权中间件写入的 `userId`。这解决用户伪造 `fromUserId` 替别人发申请的问题。

`POST /friend-requests/actions` 使用 `requestId + action` 表达接受/拒绝，应用层再次校验操作者确实是接收人，形成 Transport 与 Domain 双层保护。

### 房间 HTTP

| 文件 | 作用 |
|---|---|
| `transport/http/room/router.go` | 创建、邀请码、加入房间路由 |
| `transport/http/room/handle.go` | 绑定房间请求并调用房间应用 |
| `transport/http/room/dto.go` | 创建和加入房间的请求响应结构 |

创建和邀请接口都从认证上下文取用户；加入接口只接收短期邀请码，不直接允许客户端指定 roomId，从而把“谁可以加入”交给服务端邀请码策略。

### 消息 HTTP

| 文件 | 作用 |
|---|---|
| `transport/http/message/router.go` | 历史、离线、房间视频、弹幕查询路由 |
| `transport/http/message/handle.go` | 解析 cursor/limit/timeRange，调用消息查询用例 |
| `transport/http/message/dto.go` | HTTP 查询结果结构 |

消息发送不走 HTTP，而走 WebSocket；HTTP 只承担适合请求响应模式的历史和离线查询。

分页参数在 Handler 转换为整数，Application 再做 limit 上限约束。双层处理避免非法字符串进入业务层，也避免客户端请求极大页造成数据库和响应内存压力。

### 文件 HTTP

| 文件 | 作用 |
|---|---|
| `transport/http/file/router.go` | 普通上传、分片初始化、Part 上传、合并、文件查询路由 |
| `transport/http/file/handle.go` | 解析 multipart body/header 并构造文件应用 DTO |
| `transport/http/file/dto.go` | 分片初始化与完成的 HTTP 契约 |

上传 Part 的 `partNumber` 在 URL 中，Chunk Hash 在 Header/表单中，文件二进制保持流式 Reader 传给应用层，避免一次把大文件完整读入内存。

### 开发数据 HTTP

| 文件 | 作用 |
|---|---|
| `transport/http/testdata/router.go` | 注册开发数据重建路由 |
| `transport/http/testdata/handle.go` | 调用 Bootstrap Application 并返回结果 |

该路由由 `main.go` 按环境决定是否注册，不应仅依赖前端隐藏入口。

## 4. WebSocket Transport 文件索引

| 文件 | 作用 |
|---|---|
| `transport/ws/ws.proto` | 浏览器与服务端的二进制协议定义 |
| `transport/ws/protocol.go` | Go 内部 WebSocket 请求、消息、一起看状态结构 |
| `transport/ws/protobuf.go` | Protobuf 对象与内部结构/共享事件互转 |
| `transport/ws/router.go` | 注册带鉴权的 `/ws` 路由 |
| `transport/ws/dispatcher.go` | `op -> handler` 路由表 |
| `transport/ws/client.go` | 单条连接、发送缓冲、读写、幂等关闭 |
| `transport/ws/gateway.go` | 用户多 Session 管理、消息投递、一起看状态与控制权 |
| `transport/ws/handle.go` | 升级连接、读写循环、消息/已读/播放控制业务入口 |
| `transport/ws/pb/ws.pb.go` | Protobuf 生成文件，不手工修改 |

### 4.1 Dispatcher

```go
dispatcher.RegisterHandler("msg", handler.handleSendMessage)
dispatcher.RegisterHandler("msg_read_ack", handler.handleHistoryMessageRead)
dispatcher.RegisterHandler("watch_video_control", handler.handleWatchVideoControl)

func (d *Dispatcher) Dispatch(ctx context.Context, client *Client, op string, data []byte) {
    if handler, ok := d.handlers[op]; ok {
        _ = handler(ctx, client, data)
    }
}
```

为什么这样写：连接读循环只负责拆帧，不写巨大的 switch。增加协议操作时注册新 Handler，避免改动连接生命周期代码。

### 4.2 Client 与单写协程

```go
type Client struct {
    conn *websocket.Conn
    send chan WsMessage
    closeOnce sync.Once
}

// 业务协程只投递 channel，不直接并发写 socket。
client.send <- WsMessage{Op: op, Data: payload}

// writeLoop 是该连接唯一的普通写入者。
for msg := range client.send {
    conn.WriteMessage(websocket.BinaryMessage, marshal(msg))
}
```

Gorilla WebSocket 不允许多个协程无序并发写同一连接。单写协程保证帧完整和顺序；有界 Channel 提供背压边界。`Close` 使用 `sync.Once`，因为读循环、写循环、心跳都可能同时发现断线。

### 4.3 Gateway 多端登录

```go
sessions     map[sessionID]*Client
userSessions map[userID]map[sessionID]struct{}
```

一个用户可以在多个浏览器标签或设备建立连接。先由 userId 找到所有 sessionId，再投递每个 Client，解决“后登录连接覆盖前一个连接”的问题。删除最后一个 Session 时同时删除 userId Key，防止内存泄漏。

发送使用非阻塞 Channel：

```go
select {
case client.send <- message:
default:
    // 慢客户端不能阻塞整个 Kafka Consumer 或其他在线用户。
}
```

这是可用性优先策略：慢客户端可能丢实时推送，但可以通过离线补拉恢复。生产环境应增加丢弃指标或主动断开慢连接，不能完全静默。

### 4.4 心跳与连接生命周期

`readLoop` 设置 ReadDeadline 和 PongHandler；`writeLoop` 定期发送 Ping；Gateway KeepAlive 扫描 idle 时间。任意一侧发现连接失效都会走幂等 Close 和 RemoveClient。

解决的问题：NAT/代理已经断开但服务端仍保留连接、连接表泄漏、向僵尸连接持续发送消息。

### 4.5 一起看状态

Gateway 以 `roomId` 保存 `WatchVideoState`，当前配置 Redis 时会写 Redis，同时保留本地镜像；没有 Redis Store 时退化到进程内存。

```go
if exists && state.UpdatedBy != userID && req.Action != "get_state" {
    return ErrWatchVideoLocked
}

state = applyControl(state, req)
saveWatchState(ctx, state)
gateway.SendWatchVideoStateToUsers("watch_video_sync", members, state)
```

控制者锁避免两个人同时拖动进度导致视频来回跳。状态放 Redis 后可以跨进程读取，但在线 Session 仍是节点本地的；完整多节点推送依赖 Kafka 把事件送到每个节点对应的 Consumer 设计。

### 4.6 Protobuf

外层统一为：

```protobuf
message WsFrame {
  string op = 1;
  bytes data = 2;
}
```

先用 `op` 决定 `data` 的具体消息类型。相比 JSON，二进制体积更小、字段类型更明确；相比为每种消息建立不同 WebSocket 路径，统一连接更容易维护心跳和登录态。

## 5. MySQL 基础设施

### 入口与事务

| 文件 | 作用 |
|---|---|
| `infrastructure/persistence/mysql/entry.go` | 初始化 GORM MySQL 连接 |
| `infrastructure/persistence/tx_manager.go` | 用 GORM 实现应用层事务闭包 |

事务实现的核心是把 GORM 的 `Transaction` 隐藏在端口后：

```go
func (m *GormTxManager) WithinTransaction(ctx context.Context, fn func(*gorm.DB) error) error {
    return m.db.WithContext(ctx).Transaction(fn)
}
```

只要闭包返回 error，GORM 回滚；返回 nil 才提交。应用层因此可以清楚看到事务边界。

### MySQL Model 文件

Model 是数据库表形状，不应包含跨仓储业务编排。

| 文件 | 对应数据 |
|---|---|
| `mysql/model/user.go` | `users` 用户账号和哈希密码 |
| `mysql/model/friend.go` | `friends` 有方向好友关系 |
| `mysql/model/friend_request.go` | `friend_requests` 申请状态和时间 |
| `mysql/model/room.go` | `rooms` 房间、人数、版本 |
| `mysql/model/room_user.go` | `room_users` 成员角色与状态 |
| `mysql/model/conversation.go` | `conversations` 会话最新 seq/消息 |
| `mysql/model/user_conversation.go` | `user_conversations` 每用户读/同步游标 |
| `mysql/model/message.go` | `messages` 公共消息字段 |
| `mysql/model/message_image.go` | `message_images` 图片扩展 |
| `mysql/model/message_video.go` | `message_videos` 视频扩展 |
| `mysql/model/message_file.go` | `message_files` 文件扩展 |
| `mysql/model/message_sticker.go` | `message_stickers` 表情扩展 |
| `mysql/model/message_outbox.go` | `message_outboxes` 可靠事件记录 |
| `mysql/model/file.go` | `files` MinIO 对象元数据 |

Model 与 Domain 分离的原因：数据库可能使用 `int`、可空字段和 GORM Tag，而领域对象希望使用枚举和值对象。混用会让持久化细节进入业务规则。

### Repository 文件

每个 `converter.go` 只负责 Domain 与 Model 的字段转换；每个同名业务文件负责查询和写入。

| 文件 | 作用 |
|---|---|
| `mysql/repository/user/user.go` | 用户创建、手机号/用户名查询、批量查询、在线时间更新 |
| `mysql/repository/user/converter.go` | User Entity/Model 转换 |
| `mysql/repository/friend/friend.go` | 好友关系查询、批量创建、好友列表 |
| `mysql/repository/friend/converter.go` | Friend Entity/Model 转换 |
| `mysql/repository/friend_request/friend_request.go` | 最新申请、列表、创建、条件状态更新 |
| `mysql/repository/friend_request/converter.go` | FriendRequest Entity/Model 转换 |
| `mysql/repository/room/room.go` | 创建和查询有效房间、更新成员数/版本 |
| `mysql/repository/room/converter.go` | Room Entity/Model 转换 |
| `mysql/repository/room_user/room_user.go` | 加入/重新加入、成员关系、活跃成员 ID |
| `mysql/repository/room_user/converter.go` | RoomUser Entity/Model 转换 |
| `mysql/repository/file/file.go` | 文件元数据 Save/Get |
| `mysql/repository/file/converter.go` | File Entity/Model 转换 |
| `mysql/repository/message/message.go` | 消息保存、历史分页、各会话最新消息 |
| `mysql/repository/message/converter.go` | Message Entity/Model 转换 |
| `mysql/repository/message/conversation.go` | 会话创建、Upsert、单个/批量读取 |
| `mysql/repository/message/user_conversation.go` | 读游标、同步游标、批量更新和列表 |
| `mysql/repository/message/message_outbox.go` | Outbox 创建、加锁领取、成功/失败状态更新 |
| `mysql/repository/message/message_image.go` | 图片扩展创建和批量查询 |
| `mysql/repository/message/message_video.go` | 视频扩展创建和批量查询 |
| `mysql/repository/message/message_file_ext.go` | 文件扩展创建和批量查询 |
| `mysql/repository/message/message_sticker.go` | 表情扩展创建和批量查询 |

历史查询通常按 `(conversation_id, seq)` 过滤并倒序 Limit。这要求数据库为会话与 seq 建联合索引，否则消息量增长后会全表扫描。

会话 Upsert 解决第一条消息并发创建会话的问题；游标更新应使用只增不减的条件，避免乱序事件把 seq 写回旧值。

## 6. Outbox 与 Kafka

### Messaging 文件索引

| 文件 | 作用 |
|---|---|
| `infrastructure/messaging/entry.go` | TaskManager，将事件封装为 Envelope 并交给 MQ Client |
| `infrastructure/messaging/read_ack_outbox_worker.go` | 轮询、抢占和投递 Outbox，支持普通消息与已读事件 |
| `infrastructure/messaging/message_batcher.go` | 按会话聚合短窗口消息，批量分发的实现基础 |
| `infrastructure/messaging/client/entry.go` | MQ Client 抽象 |
| `messaging/client/kafka/entry.go` | 创建 Sarama Producer/Consumer Client |
| `messaging/client/kafka/producer.go` | 同步发送 Kafka 消息 |
| `messaging/client/kafka/consumer.go` | 持续运行 Consumer Group，Context 取消时退出 |
| `messaging/client/kafka/handler.go` | 按 Topic 处理普通消息、已读、同步 seq |
| `messaging/client/kafka/dispatch.go` | 在线 Gateway 分发适配 |
| `messaging/client/kafka/worker.go` | Kafka 工作循环相关封装 |
| `messaging/client/kafka/errors.go` | Kafka 层稳定错误 |

### 6.1 Outbox 抢占

Worker 周期性在事务中查询：

```sql
status = 'pending' AND next_retry_at <= NOW()
OR status = 'processing' AND locked_at <= stale_time
ORDER BY next_retry_at, created_at
LIMIT 50
FOR UPDATE SKIP LOCKED
```

然后把记录标记 Processing 并提交，再在事务外调用 Kafka。

为什么使用 `FOR UPDATE SKIP LOCKED`：多个 Worker 可以同时领取不同批次，不会等待彼此持有的行锁，也不会重复领取同一条任务。

为什么不在数据库事务中发送 Kafka：网络调用可能很慢。持有行锁等待 Kafka 会增加连接占用和锁竞争。先短事务抢任务，再发送，失败后更新重试时间。

```go
if err := dispatch(outbox); err != nil {
    // 指数退避，记录错误；超过锁超时也可被其他 Worker 回收。
    repository.MarkFailed(ctx, id, nextRetryAt, err.Error())
} else {
    repository.MarkSent(ctx, id, time.Now())
}
```

`Sent` 更新失败可能导致重复投递，因此下游必须幂等。可靠消息系统通常选择“至少一次”，而不是假设绝不重复。

### 6.2 Kafka Key 与顺序

```go
ProducerMessage{
    Topic: topic,
    Key:   sarama.StringEncoder(conversationID),
    Value: sarama.ByteEncoder(envelopeBytes),
}
```

同一 `conversationId` 作为 Key 会进入同一 Partition，因此该会话内保持顺序；不同会话可以并行消费。

### 6.3 Consumer ACK

```go
for msg := range claim.Messages() {
    err := handle(msg)
    if err != nil {
        // 不 Mark，Consumer Group 后续仍有机会再次处理。
        continue
    }
    session.MarkMessage(msg, "")
}
```

这就是“异步任务失败不 ACK，Kafka 会重投”的实现基础。但要注意：持续失败的毒消息会阻塞或反复消费，完整生产方案还需要重试次数和死信队列。

普通消息 Consumer：

- 私聊投递给接收用户。
- 群聊读取成员并向每个在线用户投递。
- 更新接收者 `LatestSyncSeq`。

已读事件 Consumer 将 `msg_read_ack` 转成面向原发送者的 `msg_read_notify`。

## 7. Redis 基础设施

| 文件 | 作用 |
|---|---|
| `infrastructure/persistence/redis/entry.go` | 创建 Redis Client |
| `redis/cache/key/key.go` | 统一拼接 Redis Key，过滤空片段和多余冒号 |
| `redis/cache/shared/store.go` | JSON、String Set 等通用 Redis 操作 |
| `redis/cache/auth/keys.go` | Access/Refresh Token Key 规则 |
| `redis/cache/auth/auth.go` | Token 与 userId 映射、TTL |
| `redis/cache/conversation/keys.go` | seq、成员版本、去重 Key |
| `redis/cache/conversation/conversation.go` | INCR seq、成员集合、版本和 clientMsgId 去重 |
| `redis/cache/file/keys.go` | 文件、hash、upload、part Key |
| `redis/cache/file/file.go` | 文件缓存、Multipart 元数据和已上传 Part |
| `redis/cache/room/keys.go` | 邀请码与 roomId Key |
| `redis/cache/room/room.go` | 生成、刷新和解析邀请码 |
| `redis/cache/room/errors.go` | 邀请码不存在或过期错误 |
| `redis/cache/friend/keys.go` | 好友缓存 Key |
| `redis/cache/friend/friend.go` | 好友关系缓存实现 |
| `redis/cache/user/keys.go` | 用户缓存 Key |
| `redis/cache/user/user.go` | 用户缓存实现 |

Key 构造集中解决命名冲突和不同模块手写出多种格式的问题。例如：

```text
auth:access:<token>
conversation:<id>:latest_seq
upload:<uploadId>:parts
room:invite:<code>
```

会话 seq 使用 Redis `INCR`，它是原子操作，多个服务实例同时发消息也不会拿到相同 seq。需要注意的是 Redis 成功、MySQL 事务失败时 seq 会出现空洞；IM 排序只要求单调，不要求绝对连续，因此可接受。

群成员缓存带 Room Version。成员变化时数据库版本递增，读取发现版本不同就回源重建，解决永久使用旧成员集合的问题。

## 8. MinIO、JWT、密码与 ID

### 文件索引

| 文件 | 作用 |
|---|---|
| `infrastructure/storage/minio/entry.go` | ObjectStorage 端口的 MinIO 实现，含 Multipart 和签名 URL |
| `infrastructure/security/jwt/entity.go` | JWT Claims 结构 |
| `infrastructure/security/jwt/entry.go` | JWT 生成和验证 |
| `infrastructure/security/jwt/errors.go` | Token 过期、非法等错误 |
| `infrastructure/security/auth/service.go` | 组合配置与 JWT，签发 Access/Refresh Token |
| `infrastructure/crypto/encrypt/entry.go` | MD5 工具和 BCrypt 密码哈希/验证 |
| `infrastructure/id/snow/entry.go` | Snowflake 分布式 ID 生成 |

MinIO 签名 URL：数据库保存稳定的 `ObjectKey`，每次读取文件时根据 TTL 重新签名。这样 Bucket 可以保持私有，不需要把永久公开地址存进消息。

JWT 关键模式：

```go
claims := Claims{
    UserId: userID,
    RegisteredClaims: jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(expire)),
    },
}
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
```

验证时必须同时检查签名算法和过期时间，不能只 Base64 解码 Claims。

密码使用：

```go
hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
ok := bcrypt.CompareHashAndPassword(hash, password) == nil
```

BCrypt 是单向哈希，不存在“解密原始密码”。相同密码因随机盐也会得到不同字符串。

Snowflake ID 由时间、机器号和序列组成，服务实例不依赖数据库自增主键就能生成全局趋势递增 ID。机器号必须保证实例间不重复，否则同一毫秒可能碰撞。

## 9. 基础设施阅读重点

- Repository 中首先看 WHERE 条件和索引能否对应。
- 看到缓存更新失败时，判断数据库是否已经提交以及缓存能否重建。
- 看到 Kafka `MarkMessage` 时，确认失败分支是否会误 ACK。
- 看到 Outbox 时，确认业务写入和 Outbox 创建是否确实使用同一个 `tx`。
- 看到 WebSocket 发送时，确认是否通过 `client.send` 单写协程，而不是直接并发写 Conn。
- 看到 Redis INCR 时，接受 seq 空洞，但不能接受 seq 回退或重复。

## 10. 基础设施与协议测试文件

| 文件 | 验证内容 |
|---|---|
| `infrastructure/messaging/read_ack_outbox_worker_test.go` | Worker 能分发支持的 Outbox 类型并正确标记结果 |
| `infrastructure/messaging/entry_test.go` | TaskManager 对普通消息、已读和同步 seq 的 Envelope/Topic/Key 封装 |
| `infrastructure/persistence/redis/cache/key/key_test.go` | Redis Key 过滤空片段、冒号处理和模块命名一致性 |
| `transport/ws/protobuf_test.go` | 消息、已读 Protobuf 转换、视频时间可选字段、一起看状态边界和控制者释放 |

这些测试主要验证“层与层之间的契约”，因为协议字段、Topic、Key 或可选字段一旦不一致，单个函数可能都能运行，但前后端或生产消费链路会整体失效。
