# SYCHAT

一个面向好友私域场景的实时房间协作平台。

它的产品形态更接近腾讯会议这类“房间协作”系统，只是协作内容不是语音会议，而是一起看电影、实时聊天和同步播放。用户可以通过邀请进入房间，在同一份播放状态下进行互动，并保留历史消息和观影记录。

## 项目背景

传统聊天工具只能解决“说话”，视频平台只能解决“观看”，但在好友一起看电影、边看边聊、同步进度、回看互动内容这些场景里，用户体验是割裂的。

这个项目就是为了解决这个问题：

- 让用户有自己的好友关系链
- 让用户能进入一个共享房间
- 让房间里的所有成员看到同一份视频状态
- 让聊天消息和视频时间轴关联
- 让历史聊天内容可以作为回放记录保留

从简历表达上，它更适合被描述成：

> 实时房间协作平台

而不是普通的 IM 系统。

## 项目分层

### P0 首发闭环

目标是做到：可登录、可建联、可进房、可发消息、可同步观影、可稳定运行。

#### 后端

| 状态 | 模块 | 方法 | 接口路径 | 功能 |
|---|---|---:|---|---|
| ✅ | 账户 | POST | `/api/v1/users/register` | 手机号注册 |
| ✅ | 账户 | POST | `/api/v1/users/login` | 手机号登录并获取 Token |
| ✅ | 账户 | POST | `/api/v1/users/logout` | 退出登录 |
| ✅ | 账户 | GET | `/api/v1/users/:userId` | 获取指定用户信息 |
| ✅ | 账户 | GET | `/api/v1/users/resolve` | 通过用户名或手机号解析用户 |
| ✅ | 好友 | GET | `/api/v1/friends` | 获取好友列表 |
| ✅ | 好友申请 | POST | `/api/v1/friend-requests` | 发送好友申请 |
| ✅ | 好友申请 | GET | `/api/v1/friend-requests` | 拉取好友申请列表 |
| ✅ | 好友申请 | POST | `/api/v1/friend-requests/actions` | 同意或拒绝好友申请 |
| ✅ | 消息 | GET | `/api/v1/messages/offline` | 拉取离线消息与会话摘要 |
| ✅ | 消息 | GET | `/api/v1/messages/history` | 拉取指定会话历史消息，支持游标分页 |
| ✅ | 消息 | GET | `/api/v1/ws` | WebSocket 实时收发消息、消息回执、已读确认、房间同步 |
| ✅ | 房间 | POST | `/api/v1/rooms` | 创建房间 |
| ✅ | 房间 | GET | `/api/v1/rooms/:roomId/invite-code` | 获取或刷新房间邀请码 |
| ✅ | 房间 | POST | `/api/v1/rooms/join` | 通过邀请码加入房间 |
| ✅ | 视频 / 文件 | POST | `/api/v1/files` | 普通文件上传 |
| ✅ | 视频 / 文件 | POST | `/api/v1/files/multipart/init` | 初始化分片上传 |
| ✅ | 视频 / 文件 | PUT | `/api/v1/files/multipart/:uploadId/parts/:partNumber` | 上传分片 |
| ✅ | 视频 / 文件 | POST | `/api/v1/files/multipart/:uploadId/complete` | 完成分片合并 |
| ✅ | 视频 / 文件 | GET | `/api/v1/files/:fileId` | 获取文件信息或访问地址 |
| ✅ | 视频 | GET | `/api/v1/messages/videos` | 拉取房间历史视频列表 |
| ✅ | 视频 | GET | `/api/v1/messages/danmaku` | 拉取指定房间和视频的弹幕数据 |
| ✅ | 视频 | GET | `/api/v1/ws` | 同步播放、播放控制、暂停、快进、快退、加载视频 |
| ✅ | 互动 | GET | `/api/v1/ws` | 发送聊天消息并携带视频时间上下文 |

#### 前端

| 状态 | 模块 | 方法 | 接口路径 | 功能 |
|---|---|---:|---|---|
| ✅ | 前端 | 页面 | `frontend/src/components/LoginPage.vue` | 登录 / 注册页 |
| ✅ | 前端 | 页面 | `frontend/src/components/ConversationList.vue` | 会话列表与离线消息刷新 |
| ✅ | 前端 | 页面 | `frontend/src/components/FriendsPanel.vue` | 好友申请与好友管理 |
| ✅ | 前端 | 页面 | `frontend/src/components/ChatPanel.vue` | 房间聊天与私聊发送 |
| ✅ | 前端 | 页面 | `frontend/src/components/WatchPanel.vue` | 视频上传与同步控制面板 |
| ✅ | 前端 | 逻辑 | `frontend/src/composables/useImClient.js` | WebSocket 重连、消息收发、房间状态同步 |

### P1 增强计划

这一层的目标是把“能一起看”升级成“更像真实房间协作产品”。

这些是更适合“房间协作”场景的增强方向，当前仓库里有些已经实现，有些还在规划中。

#### 后端

| 状态 | 模块 | 方法 | 接口路径 | 功能 |
|---|---|---:|---|---|
| [ ] | 房间管理 | 规划中 | - | 房间列表、房间详情、房间生命周期管理 |
| [ ] | 播放协同 | 规划中 | - | 房主控制、成员跟随、播放进度恢复、状态一致性保障 |
| [ ] | 连接恢复 | 规划中 | - | 断线重连、状态补偿、房间心跳、会话恢复 |
| [ ] | 回放增强 | 规划中 | - | 历史视频回看、观看进度续播、互动内容回放 |
| [ ] | 产品完善 | 规划中 | - | 测试、监控、限流、日志审计、异常兜底 |

#### 前端

| 状态 | 模块 | 方法 | 接口路径 | 功能 |
|---|---|---:|---|---|
| [ ] | 互动体验 | 规划中 | - | 快捷反应、时间点互动、回放标记 |
| [ ] | 交互面板 | 规划中 | - | 房间成员展示、在线状态展示、房主权限控制入口 |
| [ ] | 播放体验 | 规划中 | - | 观看进度续播、历史视频切换、回放体验优化 |

## P0 / P1 功能清单

如果你只想快速看项目做到哪一步，可以直接看这张表。

### P0

| 状态 | 功能 |
|---|---|
| ✅ | 注册 / 登录 / 退出登录 |
| ✅ | 查看当前用户信息 |
| ✅ | 好友申请 |
| ✅ | 好友列表 |
| ✅ | 接受 / 拒绝好友申请 |
| ✅ | 创建观影房间 |
| ✅ | 通过邀请码加入房间 |
| ✅ | 房间内实时聊天 |
| ✅ | WebSocket 实时消息收发 |
| ✅ | 视频文件上传 |
| ✅ | 分片上传视频 |
| ✅ | 加载视频到房间 |
| ✅ | 播放 / 暂停 / 拖动进度同步 |
| ✅ | 房间历史聊天记录 |
| ✅ | 房间历史视频记录 |
| ✅ | 弹幕回放 |

### P1

| 状态 | 功能 |
|---|---|
| [ ] | 房间列表与房间详情 |
| [ ] | 观影状态恢复 |
| [ ] | 房主权限控制 |
| [ ] | 成员在线状态显示 |
| [ ] | 房主踢人 / 禁言 |
| [ ] | 快捷表情 / 快捷反应 |
| [ ] | 时间点互动标记 |
| [ ] | 视频切换历史 |
| [ ] | 更完整的回放体验 |
| [ ] | 测试、监控、限流、日志审计 |

## 技术栈

- Go 1.25.2
- Gin
- GORM
- MySQL 8
- Redis 7
- Kafka 3.9
- MinIO
- Vue 3
- Vite

## 系统能力

后端当前已经接入的基础设施包括：

- MySQL：用户、好友、房间、消息、文件等持久化
- Redis：登录态、会话缓存、房间缓存、文件缓存
- Kafka：消息投递与消费
- MinIO：视频文件对象存储
- WebSocket：实时消息和房间同步通道

## 后端测试

当前仓库已有少量后端测试，主要覆盖基础协议、文件链路、消息发送与缓存 key 规则。

- [✅] `backend/internal/transport/ws/protobuf_test.go` - WebSocket 协议编解码、消息事件、观影状态边界
- [✅] `backend/internal/infrastructure/messaging/entry_test.go` - 消息同步序列发送
- [✅] `backend/internal/application/file/file_test.go` - 文件上传、缓存命中、URL 刷新、空文件拒绝
- [✅] `backend/internal/infrastructure/persistence/redis/cache/key/key_test.go` - Redis key 生成规则

说明：

- 当前测试数量不多，主要用于验证核心链路和基础工具函数
- 如果依赖服务未启动，部分真实接口仍需要手工验证

## 目录结构

```text
.
├── backend
│   ├── cmd
│   │   ├── migrate
│   │   └── server
│   ├── configs
│   └── internal
├── frontend
├── docs
├── package.json
└── README.md
```

## 本地启动

### 1. 创建 Docker 网络

```bash
docker network create IMChat-net
```

### 2. 启动依赖服务

```bash
docker compose -f backend/docker-compose.yaml up -d
```

### 3. 启动后端

```bash
cd backend
go run ./cmd/server
```

默认监听：

```text
http://localhost:8081
```

## 认证方式

除注册、登录外，大部分 HTTP 接口都需要携带 JWT。

请求头格式：

```text
Authorization: Bearer <access_token>
```

登录成功后：

- 响应体中返回 `token`
- 服务端会写入 `refresh_token` Cookie
- Redis 中会保存当前登录态映射

登录支持两种方式：

- 手机号登录
- 用户名登录

## 已知限制

- 配置文件路径当前在启动入口中写死
- `backend/cmd/migrate/main.go` 仍是占位状态
- 项目缺少系统性的单元测试与集成测试
- P1 的协同观影增强能力还没有真正补齐

## 简历表达建议

如果你准备投简历，推荐把这个项目写成：

> 实时房间协作平台。支持用户登录、好友关系链、房间创建/加入、视频上传与分片分发、WebSocket 实时聊天、同步播放控制与回放记录。后端采用 Gin + GORM + MySQL + Redis + Kafka + MinIO 构建，负责消息可靠投递、状态同步和文件分发。
