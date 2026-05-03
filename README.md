# IMChat

一个基于 Golang 的即时通讯后端服务，当前包含用户、好友、好友申请、群聊房间、历史消息查询与 WebSocket 实时消息能力。

## 项目状态

当前仓库以后端为主，前端工程尚未初始化完成。

后端已具备以下能力：

- 用户注册、登录、登出、查询用户信息
- 好友申请、好友列表
- 群聊创建、邀请、入群
- 历史消息与离线消息拉取
- 基于 WebSocket 的实时消息收发
- 基于 Redis 的登录态校验
- 基于 Kafka 的消息投递与消费

当前仓库也有一些明显的在建特征：

- 根目录 `package.json` 仍为空对象，前端尚未落地
- `backend/cmd/migrate/main.go` 还是占位文件，迁移命令尚未实现
- 项目缺少单元测试与集成测试
- 配置文件路径目前在服务启动入口中被写死

## 技术栈

- Go 1.25.2
- Gin
- GORM
- MySQL 8
- Redis 7
- Kafka 3.9

## 目录结构

```text
.
├── backend
│   ├── cmd
│   │   ├── migrate
│   │   └── server
│   ├── configs
│   └── internal
│       ├── apis
│       ├── applications
│       ├── domain
│       ├── infrastructure
│       ├── port
│       └── protocol
├── package.json
└── README.md
```

主要目录说明：

- `backend/cmd/server`：服务启动入口
- `backend/configs`：YAML 配置与配置加载
- `backend/internal/apis/https`：HTTP 接口
- `backend/internal/apis/ws`：WebSocket 协议与处理器
- `backend/internal/applications`：应用服务层
- `backend/internal/domain`：领域模型
- `backend/internal/infrastructure`：数据库、缓存、消息队列、JWT 等基础设施
- `backend/internal/protocol`：内部消息协议定义

## 环境依赖

启动前需要准备：

- Go 1.25.2 或兼容版本
- Docker 与 Docker Compose
- 一个名为 `IMChat-net` 的 Docker 网络

项目默认依赖以下服务：

- MySQL：`mysql:3306`
- Redis：`redis:6379`
- Kafka：`kafka:9092`

## 配置说明

默认配置文件位于 [backend/configs/config.yaml](/workspace/IM/backend/configs/config.yaml:1)。

服务启动时当前写死读取：

```go
cfg := configs.LoadConfig("/workspace/IM/backend/configs/config.yaml")
```

也就是说，如果你修改目录结构或在其他路径运行，需要同步调整 [backend/cmd/server/main.go](/workspace/IM/backend/cmd/server/main.go:1)。

默认关键配置：

- 服务监听端口：`8081`
- MySQL DSN：`user:123456@tcp(mysql:3306)/IMChat?...`
- Redis DSN：`redis://redis:6379/1?...`
- Kafka Broker：`kafka:9092`
- JWT Access Token 过期时间：15 分钟
- JWT Refresh Token 过期时间：168 小时

## 快速开始

### 1. 创建 Docker 网络

```bash
docker network create IMChat-net
```

### 2. 启动基础依赖

```bash
docker compose -f backend/docker-compose.yaml up -d
```

### 3. 启动后端服务

```bash
cd backend
go run ./cmd/server
```

服务默认监听：

```text
http://localhost:8081
```

## 开发验证

当前可以先做最基本的编译校验：

```bash
cd backend
go test ./...
```

说明：

- 当前仓库几乎没有测试文件，因此这一步主要用于确认可以编译通过
- 如果依赖服务未启动，某些真实接口在运行时仍会失败

## 认证方式

除注册和登录接口外，大部分 HTTP 接口都要求携带 JWT。

请求头格式：

```text
Authorization: Bearer <access_token>
```

中间件会做两层校验：

- 校验 JWT 签名与过期时间
- 从 Redis 中校验当前 token 与用户 ID 的映射关系

登录与注册成功后：

- 响应体中会返回 `token`
- 服务端还会写入 `refresh_token` Cookie

## HTTP API

统一响应格式：

```json
{
  "code": 200,
  "message": "success",
  "data": {}
}
```

### 用户

#### `POST /api/v1/user/register`

请求体：

```json
{
  "phone": "13800000000",
  "password": "123456",
  "reconfirmPassword": "123456",
  "loginType": 1
}
```

说明：

- `loginType=1` 表示手机号注册
- 微信注册预留但未实现

#### `POST /api/v1/user/login`

请求体：

```json
{
  "loginType": 1,
  "phone": "13800000000",
  "password": "123456"
}
```

当前实现情况：

- `loginType=1`：手机号登录已实现
- `loginType=2`：用户名登录未实现
- `loginType=3`：微信登录未实现

#### `POST /api/v1/user/logout`

需要认证。

#### `GET /api/v1/user/:userId`

需要认证。

### 好友

#### `GET /api/v1/friend`

获取好友列表，需要认证。

### 好友申请

#### `POST /api/v1/friend/request`

请求体：

```json
{
  "toUserId": "target-user-id",
  "message": "你好，我想加你为好友"
}
```

#### `GET /api/v1/friend/request/:userId`

获取好友申请列表，需要认证。

说明：

- 当前实现中实际使用的是登录态里的 `userId`
- 路径参数 `:userId` 目前没有真正参与查询逻辑

#### `POST /api/v1/friend/request/opreate`

请求体：

```json
{
  "requestId": "request-id",
  "action": 1
}
```

说明：

- `action=1`：接受
- `action=2`：拒绝
- 路径中的 `opreate` 为当前代码中的既有拼写

### 群聊房间

#### `POST /api/v1/room/create`

请求体：

```json
{
  "roomName": "技术群",
  "description": "项目讨论",
  "avatar": "https://example.com/room.png"
}
```

#### `GET /api/v1/room/invite/:roomId`

获取入群邀请码，需要认证。

#### `POST /api/v1/room/join`

请求体：

```json
{
  "inviteCode": "ABC123"
}
```

### 消息

#### `GET /api/v1/messages/history`

查询参数：

- `conversationId`：必填
- `cursor`：可选
- `limit`：可选

示例：

```text
/api/v1/messages/history?conversationId=conv-id&cursor=0&limit=20
```

#### `GET /api/v1/messages/offline`

获取离线消息摘要，需要认证。

## WebSocket

连接地址：

```text
ws://localhost:8081/api/v1/ws
```

连接要求：

- 必须携带 `Authorization: Bearer <access_token>`
- 连接建立前同样会经过 JWT 中间件校验

客户端发包格式：

```json
{
  "op": "msg",
  "data": {
    "clientMsgId": "local-msg-id",
    "recvId": "target-id",
    "convType": 1,
    "cType": 1,
    "content": "hello"
  }
}
```

字段说明：

- `op=msg`：发送消息
- `op=msg_read_ack`：消息已读回执

消息发送 `data` 字段：

- `clientMsgId`：客户端本地消息 ID
- `recvId`：接收方 ID，单聊时一般为用户 ID，群聊时一般为房间 ID
- `convType`：会话类型
- `cType`：消息内容类型
- `content`：消息内容
- `videoTime`：视频消息时长，可选

会话类型：

- `1`：单聊
- `2`：群聊

服务端 ACK 事件示例：

```json
{
  "type": "msg_ack",
  "data": {
    "clientMsgId": "local-msg-id",
    "messageId": "server-msg-id",
    "status": "sent",
    "extra": "",
    "sendTime": 0
  }
}
```

服务端消息推送事件类型：

- `msg`
- `msg_ack`
- `msg_read_ack`

## Docker Compose 说明

[backend/docker-compose.yaml](/workspace/IM/backend/docker-compose.yaml:1) 当前包含：

- `redis`
- `mysql`
- `kafka`

需要注意：

- Compose 使用的是外部网络 `IMChat-net`
- `mysql-data` 卷已声明
- `kafka-data` 卷已声明但当前服务配置里没有挂载使用

## 已知问题

当前代码中有一些值得提前知道的限制：

- 根目录前端尚未实现
- 配置文件路径被写死，不利于多环境部署
- 数据库迁移入口还未实现
- HTTP 路由与部分命名存在拼写问题，例如 `opreate`
- 好友申请列表接口带了 `:userId` 参数，但当前处理逻辑未使用
- 仓库缺少自动化测试
- README 中记录的是当前实现，不代表接口设计已经稳定

## 后续建议

如果你准备继续开发，优先建议补这几项：

1. 实现数据库迁移命令
2. 去掉配置文件硬编码，改为环境变量或命令行参数注入
3. 补充用户、消息、房间相关测试
4. 整理并修正已有路由命名
5. 增加 OpenAPI/Swagger 文档或接口示例集合
