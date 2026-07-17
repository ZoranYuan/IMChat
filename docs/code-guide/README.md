# SYCHAT 项目代码导读

这套文档面向第一次系统阅读本项目代码的开发者。它不重复配置文件，也不逐行翻译代码，而是回答四个问题：

1. 每个业务文件负责什么。
2. 核心调用链从哪里进入、经过哪些层、最后写到哪里。
3. 关键代码为什么这样写。
4. 这种写法解决了什么业务或工程问题，还有什么边界。

## 文档范围

文档覆盖：

- `backend/cmd/server` 的依赖组装。
- `backend/internal/domain` 领域对象和值对象。
- `backend/internal/application` 应用服务、DTO、错误和端口接口。
- `backend/internal/transport` HTTP 与 WebSocket 入口。
- `backend/internal/infrastructure` MySQL、Redis、Kafka、MinIO、JWT、ID 等实现。
- `frontend/src` 下所有 Vue 组件、组合式函数、API 与 WebSocket 协议代码。

文档不展开：

- `backend/configs`、`vite.config.js` 等配置代码。
- `backend/internal/transport/ws/pb/ws.pb.go` 这类生成代码。
- CSS 视觉样式的逐条解释。
- Dockerfile、Compose、Nginx 等部署配置。

测试文件会在相关模块中说明测试目标，但不会逐行讲解测试桩。

## 阅读顺序

建议按以下顺序阅读：

1. [后端领域与应用层](./backend-domain-application.md)
2. [后端接口与基础设施](./backend-transport-infrastructure.md)
3. [前端业务代码](./frontend.md)
4. 回到真实源码，按下面的核心链路打断点或加日志。

## 架构分层

```text
前端 Vue
  | HTTP: 登录、好友、历史、文件、房间
  | WebSocket + Protobuf: 消息、ACK、已读、一起看
  v
Transport: Gin Handler / WS Handler
  v
Application: 编排用例、事务、权限、DTO
  v
Domain: User / Message / Conversation / Room / Friend
  v
Ports: Repository / Cache / MQ / ObjectStorage 接口
  v
Infrastructure: MySQL / Redis / Kafka / MinIO / JWT
```

这样分层的核心价值是让业务规则不直接依赖 Gin、GORM、Redis 或 Kafka。应用层依赖接口，基础设施层实现接口，因此核心用例可以用 Fake Repository 做单元测试。

## 五条核心链路

### 1. 发送消息

```text
ChatPanel
  -> useConversation.sendMessage/sendMediaPayload
  -> WebSocket msg
  -> WSHandler.handleSendMessage
  -> MessageApplication.HandleMessage
  -> MySQL 事务写 messages/conversations/user_conversations/outbox
  -> Outbox Worker 发布 Kafka
  -> Kafka GroupHandler
  -> Gateway.SendToUsers
  -> 接收方 useConversation.handleIncomingMessage
```

关键点是“业务数据和 Outbox 同事务”。数据库提交成功时，待发送事件一定存在；Kafka 暂时不可用时由 Worker 重试，而不是丢消息。

### 2. 已读回执

```text
用户进入会话
  -> 拉历史完成
  -> 前端发送 msg_read_ack(lastReadSeq)
  -> 更新 user_conversations.last_read_seq
  -> 同事务写 read-ack outbox
  -> Kafka
  -> 通知原发送者 msg_read_notify
```

`lastReadSeq` 只允许向前推进，重复 ACK 会直接返回，因此天然具备幂等性。

### 3. 离线与重连

```text
WebSocket 断线
  -> 指数退避重连
  -> 重连成功调用 loadOffline
  -> 后端比较 conversation.latest_seq、last_read_seq、latest_sync_seq
  -> 返回会话最新消息和未读数
  -> 前端按 conversationId 合并，而不是覆盖当前正在看的数据
```

这里把“读到哪里”和“同步到哪里”分开，分别解决未读统计与离线补数据问题。

### 4. 媒体消息

```text
选择图片/视频/文件
  -> 上传 MinIO，MySQL 保存 files 元数据
  -> WebSocket 发送 fileId、URL、尺寸、时长和文字说明
  -> messages 保存公共字段
  -> message_images/message_videos/message_files 保存类型专属字段
```

主表加子表避免 `messages` 出现大量只对某一种类型有效的空列，同时让每种媒体类型可以独立扩展。

### 5. 一起看

```text
WatchPanel 原生播放事件
  -> useRoomWatch.sendWatchControl
  -> WebSocket watch_video_control
  -> 校验房间成员和当前控制者
  -> Redis/内存保存播放状态
  -> 广播 watch_video_sync
  -> 其他客户端 applyWatchState
```

服务端作为播放状态仲裁者，避免客户端互相直连后出现多份冲突状态。

## 阅读方法

- 先找 Transport 入口，确认请求字段是谁提供的。
- 再看 Application，关注事务边界、权限判断和跨仓储编排。
- 然后看 Entity，确认状态变化是否由领域对象约束。
- 最后看 Repository/Cache/MQ 实现，理解一致性和性能策略。
- 遇到 `seq` 时同时区分 `latestSeq`、`lastReadSeq` 和 `latestSyncSeq`，不要混为一个字段。

## 当前已知边界

- HTTP 登录入口目前只开放手机号登录，微信/GitHub 仍是预留入口。
- 表情包消息协议和后端子表已支持，但前端没有表情选择器。
- 语音按钮没有对应业务实现。
- Outbox Worker 名称仍叫 `OutboxWorker`，实际已经同时分发普通消息和已读事件。
- 历史分页首次查询会结合 `LastReadSeq`，它不是传统意义上永远从最新消息开始的分页，阅读时要注意这个产品语义。
- 当前会话实时收到新消息时前端会保持未读为 0，但不会立即发送新的已读 ACK；ACK 主要在进入会话并拉完历史后发送，这一处仍需要完善。
- 部分错误转换和 TODO 仍可继续收敛，文档会按当前代码解释，不把预留能力描述成已完成能力。
