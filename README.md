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
- Vue 3
- Vite

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

## 接口文件

- `docs/apipost.collection.json`：Apipost 可直接导入的接口集合文件
