# SYCHAT

一个面向群聊场景的实时房间协作平台。

## 项目背景

传统聊天工具只能解决“交流”，视频平台只能解决“观看”，但在好友一起看视频、边看边聊、同步进度、回放互动内容这些场景里，用户体验通常是割裂的。

这个项目就是为了解决这类问题：

- 让用户具备好友关系链
- 让用户能够进入一个共享房间
- 让房间成员看到同一份视频状态
- 让聊天消息和视频时间轴关联
- 让历史消息和历史视频都可以回放

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

| 功能模块 | 模块说明 | 对应接口 |
|---|---|---|
| 账户模块 | 用户注册、登录、退出和身份信息查询 | `POST /users/register`<br>`POST /users/login`<br>`POST /users/logout`<br>`GET /users/:userId`<br>`GET /users/resolve` |
| 好友模块 | 用户搜索、好友列表和好友申请管理 | `GET /friends`<br>`POST /friend-requests`<br>`GET /friend-requests`<br>`POST /friend-requests/actions` |
| 聊天模块 | 私聊、群聊、历史消息和离线消息 | `GET /messages/history`<br>`GET /messages/offline` |
| 房间模块 | 房间创建、加入和邀请码管理 | `POST /rooms`<br>`GET /rooms/:roomId/invite-code`<br>`POST /rooms/join` |
| 视频模块 | 视频上传、分片上传、加载和查询 | `POST /files`<br>`POST /files/multipart/init`<br>`PUT /files/multipart/:uploadId/parts/:partNumber`<br>`POST /files/multipart/:uploadId/complete`<br>`GET /files/:fileId` |
| 一起看模块 | 房间内视频同步播放与共享控制 | `GET /ws` |
| 回放模块 | 房间历史视频、弹幕回放与互动记录 | `GET /messages/videos`<br>`GET /messages/danmaku` |
| 实时通道 | 实时消息收发、已读确认和播放状态同步 | `GET /ws` |

## 功能清单

- 手机号注册与登录
- 好友搜索与好友申请
- 私聊与群聊
- 离线消息与历史消息
- WebSocket 实时通信
- 创建房间与加入房间
- 群聊内一起看入口
- 视频上传与加载
- 房间内视频同步播放
- 历史视频预览与恢复
- 房间历史消息回放
- 弹幕回放

## 接口文件

- `docs/apipost.collection.json`：Apipost 可直接导入的接口集合文件
