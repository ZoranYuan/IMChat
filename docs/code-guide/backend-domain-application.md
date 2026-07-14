# 后端领域与应用层

## 1. 这一层负责什么

`domain` 表达业务对象本身的规则，`application` 表达一个完整用例如何调用多个对象和外部端口。

例如“接受好友申请”不是一条 SQL：它需要校验申请状态、更新申请、双向建立好友关系，并保证这些写操作一起成功或一起失败。因此状态转换放在 Entity，事务编排放在 Application。

## 2. Domain 文件索引

### 用户域

| 文件 | 作用 | 重要内容 |
|---|---|---|
| `domain/user/entity/user.go` | 用户实体及注册、登录规则 | `RegisterWithPhone`、`LoginWithPhone` |
| `domain/user/entity/errors.go` | 用户领域错误 | 手机号、密码等规则错误 |
| `domain/user/value_object/phone.go` | 手机号值对象 | 中国大陆手机号格式校验 |
| `domain/user/value_object/password.go` | 密码值对象 | BCrypt 生成与验证 |
| `domain/user/value_object/loginType.go` | 登录类型枚举 | 手机、微信、GitHub 等类型边界 |
| `domain/user/value_object/status.go` | 用户状态枚举 | 激活、冻结等状态语义 |

关键代码：

```go
func RegisterWithPhone(phone uservo.Phone, password uservo.Password) (*User, error) {
    // 先由值对象校验，非法数据不能进入实体。
    if !phone.Validate() {
        return nil, ErrInvalidPhoneNumber
    }

    // 数据库永远只保存 BCrypt 哈希，不保存原始密码。
    hashedPassword, err := password.GenPasswordHash()
    if err != nil {
        return nil, err
    }

    return &User{
        Phone:      phone,
        Password:   hashedPassword,
        LoginType:  uservo.PhoneType,
        Status:     uservo.StatusActivate,
        UserName:   string(phone),
        OnLineTime: time.Now(),
    }, nil
}
```

为什么这样写：

- 手机号和密码不再只是没有语义的 `string`，规则集中在值对象中。
- 注册实体只能通过构造函数产生，防止 Handler 忘记校验或忘记哈希密码。
- BCrypt 自带随机盐和计算成本，泄露数据库后也不能直接反推出原始密码。

解决的问题：输入校验散落、明文密码泄露、不同入口执行不同注册规则。

> 注意：当前 `LoginWithPhone` 在密码不匹配时返回的领域错误仍是 `ErrInvalidPhoneNumber`，从命名和 HTTP 映射角度看应继续统一为“密码错误”。文档描述的是现状，不表示这是最佳状态。

### 好友域

| 文件 | 作用 | 重要内容 |
|---|---|---|
| `domain/friend/entity/friend.go` | 单向好友记录；双向关系由两条记录组成 | `NewFriendRelation`、`CanAdd` |
| `domain/friend/entity/errors.go` | 好友规则错误 | 不能添加自己等错误 |
| `domain/friend/value_object/status.go` | 好友关系状态 | 好友、删除、拉黑等语义 |
| `domain/friend_request/entity/friend_request.go` | 好友申请状态机 | 创建、重新申请、接受、拒绝 |
| `domain/friend_request/entity/errors.go` | 申请状态机错误 | 重复操作、越权、频繁申请 |
| `domain/friend_request/value_object/status.go` | 申请状态枚举 | Pending、Accepted、Refused |

关键代码：

```go
func (fq *FriendRequest) Accept(operatorID string) error {
    // 只有待处理申请允许接受，防止重复点击造成重复建好友。
    if fq.Status != friendrequestvo.Pending {
        return ErrDuplicateOperation
    }

    // 只有接收申请的人能操作，申请发起人不能替对方接受。
    if fq.ToUserId != operatorID {
        return ErrInvalidOperation
    }

    fq.Status = friendrequestvo.Accepted
    return nil
}
```

为什么这样写：状态转换属于申请对象，而不是 Controller 中的 `if`。未来增加“过期”“撤回”等状态时，可以在一个位置维护合法转换。

解决的问题：重复接受、越权操作、状态判断分散以及并发下难以推断最终状态。

### 房间域

| 文件 | 作用 | 重要内容 |
|---|---|---|
| `domain/room/entity/room.go` | 群聊/一起看房间实体 | 房间创建和可邀请判断 |
| `domain/room/entity/room_user.go` | 用户与房间的成员关系 | 加入、重新加入、离开、邀请权限 |
| `domain/room/entity/errors.go` | 房间领域错误 | 重复加入、权限不足等 |
| `domain/room/value_object/role.go` | 房间角色 | 房主、普通成员 |
| `domain/room/value_object/room_status.go` | 房间状态 | 正常、停用等 |
| `domain/room/value_object/room_user_status.go` | 成员状态 | 正常、禁言、离开、被踢 |

关键代码：

```go
func (ru *RoomUser) ReJoin() error {
    // 只有已经离开或被踢的成员记录才允许复用。
    if ru.Status != roomvo.BeKicked && ru.Status != roomvo.Left {
        return ErrDuplicateJoin
    }

    ru.Status = roomvo.Activate
    ru.JoinTime = time.Now().UnixMilli()
    ru.MuteUtil = nil
    ru.LeaveTime = nil
    return nil
}
```

为什么这样写：保留成员关系记录再做状态变化，比直接删除记录更适合审计、重新入群和历史消息权限判断。

解决的问题：重复加入、成员历史丢失、重新加入时残留离开时间或禁言状态。

### 消息域

| 文件 | 作用 | 重要内容 |
|---|---|---|
| `domain/message/entity/message.go` | 消息公共主实体 | 会话、发送人、seq、类型、正文、视频时间点 |
| `domain/message/entity/conversation.go` | 会话聚合信息 | 私聊/群聊参与方、最新消息和最新 seq |
| `domain/message/entity/user_conversation.go` | 用户在会话中的游标 | `LastReadSeq`、`LatestSyncSeq`、免打扰 |
| `domain/message/entity/message_image.go` | 图片消息扩展 | fileId、缩略图、尺寸、大小、URL |
| `domain/message/entity/message_video.go` | 视频消息扩展 | fileId、封面、时长、尺寸、URL |
| `domain/message/entity/message_file.go` | 文件消息扩展 | 文件名、MIME、大小、URL |
| `domain/message/entity/message_sticker.go` | 表情包扩展 | stickerId、packId、尺寸、URL |
| `domain/message/entity/message_outbox.go` | 待投递事件 | 状态、重试次数、锁、下次重试时间 |
| `domain/message/entity/errors.go` | 消息领域错误 | 消息内容和状态错误 |
| `domain/message/value_object/ctype.go` | 内容类型 | Text、Image、Video、Sticker、File |
| `domain/message/value_object/conv_type.go` | 会话类型 | PrivateChat、RoomChat |
| `domain/message/value_object/status.go` | 消息状态 | Normal 等状态 |

会话 ID 的关键代码：

```go
func GetConversationID(senderID, targetID string, convType int) string {
    if convType == int(messagevo.PrivateChat) {
        // A 发给 B 和 B 发给 A 必须得到相同 ID。
        return max(targetID, senderID) + "_" + min(targetID, senderID)
    }

    // 群聊直接使用 roomId，一间房只有一个消息序列。
    return targetID
}
```

为什么这样写：私聊会话 ID 与发送方向无关，双方消息才能落入同一条时间线；群聊以房间为天然聚合键。

解决的问题：A->B 和 B->A 生成两个会话、历史消息无法合并、Kafka 无法按同一会话 Key 保序。

三个序列字段必须区分：

| 字段 | 所属对象 | 含义 |
|---|---|---|
| `LatestSeq` | Conversation | 会话当前已经产生的最大消息序号 |
| `LastReadSeq` | UserConversation | 该用户明确读到的位置，用于未读数 |
| `LatestSyncSeq` | UserConversation | 服务端已为该用户同步到的位置，用于重连补拉 |

媒体子表的关键设计：

```go
type Message struct {
    MessageId      string
    ConversationId string
    SendId         string
    Seq            int64
    Type           messagevo.CType
    Content        string // 文本或媒体说明文字
}

type MessageVideo struct {
    MessageId  string // 与 messages 一对一关联
    FileId     string
    DurationMs int64
    Width      int
    Height     int
    URL        string
}
```

为什么这样写：`messages` 只承担所有消息共有的排序和检索字段，类型专属信息进入子表。

解决的问题：主表大量空列、每新增一种消息类型就修改主表、历史查询无法先快速只取公共字段。

Outbox 实体中的 `Pending -> Processing -> Sent` 状态解决多 Worker 抢任务和失败重试问题；`LockedAt` 让 Worker 崩溃后可回收长期停留在 Processing 的任务。

### 文件域

| 文件 | 作用 | 重要内容 |
|---|---|---|
| `domain/file/entity/file.go` | 文件元数据实体 | 对象桶、对象 Key、原始文件名、类型、大小、临时 URL |

MinIO 保存二进制，MySQL 的 `File` 只保存可检索元数据和对象引用。这样数据库不承担大对象 IO，URL 过期后还能根据 `ObjectKey` 重新签名。

## 3. Application 文件索引

### 用户应用

| 文件 | 作用 |
|---|---|
| `application/user/user.go` | 注册、登录、签发 Token、查用户、退出登录用例 |
| `application/user/dto.go` | 向 Transport 输出的用户 DTO，隔离数据库 Model |
| `application/user/errors.go` | 应用层稳定错误，供 HTTP 层映射状态码 |

关键代码：

```go
func (ua *UserApplication) issueTokensAndCache(userID string) (string, string, error) {
    access, refresh, err := ua.authService.IssueToken(userID)
    if err != nil {
        return "", "", err
    }

    // 缓存写设置超时，避免 Redis 故障无限阻塞登录请求。
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    ua.authCache.SetAccessToken(ctx, access, userID, accessTTL)
    ua.authCache.SetRefreshToken(ctx, refresh, userID, refreshTTL)
    return access, refresh, nil
}
```

为什么这样写：Token 的生成由 AuthService 抽象，缓存由 AuthCache 抽象，用户应用只负责编排。这使 JWT 或 Redis 可以替换，也便于单元测试。

解决的问题：应用服务直接依赖第三方库、登录请求被外部缓存拖死、Token 生命周期散落在 Handler。

注册流程依次做“查重 -> 密码确认 -> 领域构造 -> Snowflake ID -> 入库 -> 发 Token”。这个顺序保证无效密码不会写数据库，只有用户持久化成功后才签发身份。

### 好友与好友申请应用

| 文件 | 作用 |
|---|---|
| `application/friend/friend.go` | 获取好友列表，并补充好友用户信息 |
| `application/friend/dto.go` | 好友展示 DTO |
| `application/friend/errors.go` | 已是好友等应用错误 |
| `application/friend_request/friend_request.go` | 申请、接受、拒绝、申请列表用例 |
| `application/friend_request/dto.go` | 好友申请展示 DTO |
| `application/friend_request/errors.go` | 频繁申请、越权、重复操作等应用错误 |

接受好友申请的核心代码：

```go
record.Accept(operatorID) // 先执行领域状态校验

err := txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
    requestRepo := friendRequestRepository.WithTx(tx)
    friendRepo := friendRepository.WithTx(tx)

    // 条件更新 Pending -> Accepted，抵抗并发重复提交。
    if err := requestRepo.OperateRequest(record.RequestId, Pending, Accepted); err != nil {
        return err
    }

    // 好友列表按用户查询，所以保存两条有方向的关系。
    return friendRepo.Create([]Friend{
        {UserId: from, FriendUserId: to},
        {UserId: to, FriendUserId: from},
    })
})
```

为什么这样写：申请状态更新与双向好友记录必须处于同一事务，不能出现“申请显示已接受但好友列表没有对方”。

事务成功后再 Best Effort 预热会话成员缓存。缓存失败不会回滚数据库，因为缓存可重建，数据库才是事实来源。

### 房间应用

| 文件 | 作用 |
|---|---|
| `application/room/room.go` | 创建房间、生成邀请码、通过邀请码加入房间 |
| `application/room/dto.go` | 房间和成员返回 DTO |
| `application/room/errors.go` | 房间不存在、邀请码失效、并发更新等错误 |

创建房间的关键事务：

```go
err := txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
    // 四条数据必须形成一个完整房间。
    if err := roomRepo.WithTx(tx).Create(room); err != nil { return err }
    if _, err := roomUserRepo.WithTx(tx).JoinRoom(ownerMember); err != nil { return err }
    if err := conversationRepo.WithTx(tx).CreateConversation(ctx, conversation); err != nil { return err }
    return userConversationRepo.WithTx(tx).CreateUserConversation(ctx, ownerConversation)
})
```

为什么这样写：房间、房主成员关系、群会话、房主的会话游标缺一不可，事务保证用户不会看到“创建了一半”的房间。

邀请码保存在 Redis 并带 TTL，适合短生命周期、允许刷新且不需要永久审计的数据。加入房间时仍回到 MySQL 校验房间状态，防止只信任缓存。

### 文件应用

| 文件 | 作用 |
|---|---|
| `application/file/file.go` | 普通上传、分片初始化、上传分片、合并、查询文件 |
| `application/file/dto.go` | 上传请求、分片状态和文件响应 DTO |
| `application/file/errors.go` | 缺文件、非法分片、未完成、越权等错误 |

普通上传关键代码：

```go
// 1. 二进制先写对象存储。
storage.PutObject(ctx, objectKey, reader, size, contentType)

// 2. 生成短期可访问 URL。
url := storage.PresignedGetURL(ctx, objectKey, urlTTL)

// 3. MySQL 保存长期引用，Redis 缓存查询结果。
file := fileentity.NewFile(fileID, uploaderID, bucket, objectKey, name, contentType, size, url)
repository.Save(ctx, file)
cache.Set(ctx, file, cacheTTL)
```

为什么先写 MinIO 再写元数据：数据库中不会出现指向不存在对象的成功记录。代价是数据库保存失败时可能产生孤儿对象，生产环境应增加对象清理任务。

分片上传关键代码：

```go
// fileHash 已完成：直接返回已有 fileId，实现秒传。
if fileID := cache.GetFileIdByHash(ctx, dto.FileHash); fileID != "" {
    return completedResult(fileID)
}

// 同一个 hash 有活跃 uploadId：返回已上传 part，支持刷新后续传。
if uploadID := cache.GetActiveUploadId(ctx, dto.FileHash); uploadID != "" {
    return resultWithUploadedParts(uploadID)
}

// 否则在 MinIO 创建 Multipart Upload，并把会话状态写 Redis。
uploadID := storage.CreateMultipartUpload(ctx, objectKey, contentType)
cache.SetMultipartUpload(ctx, meta, 24*time.Hour)
```

合并前会检查分片数量、分片号连续性和 ETag。ETag 是 MinIO 确认每个 Part 的依据，仅仅相信前端“上传完成”是不够的。

解决的问题：大文件一次上传容易超时、网络中断需要从头开始、同一文件重复占用存储和带宽。

### 消息应用

| 文件 | 作用 |
|---|---|
| `application/message/message.go` | 消息域核心：发送、权限、已读、历史、离线、媒体聚合 |
| `application/message/dto.go` | 消息、会话、离线结果 DTO |
| `application/message/events.go` | 应用内部事件数据结构 |
| `application/message/errors.go` | 非好友、非群成员、保存失败等稳定错误 |

#### 3.1 会话成员校验与缓存击穿保护

```go
isMember, version, err := cache.IsMemberWithVersion(ctx, conversationID, userID)
if err == nil && version > 0 && convType == PrivateChat {
    return isMember, nil
}

// 同一用户、同一会话的并发回源合并成一次数据库查询。
value, err, _ := sf.Do(userID+":"+conversationID, func() (any, error) {
    // Double Check：等待期间其他请求可能已经填好缓存。
    if member, version, _ := cache.IsMemberWithVersion(...); version > 0 {
        return member, nil
    }

    // 私聊查好友关系；群聊查活跃成员列表，再回填缓存。
    return loadMembershipFromDBAndWarmCache()
})
```

为什么这样写：热门群缓存失效时，大量消息请求可能同时查询 MySQL。`singleflight` 将相同 Key 的回源合并，Double Check 避免重复工作；版本号让群成员变化后旧缓存可以失效。

解决的问题：缓存击穿、已退群成员仍能发消息、私聊绕过好友关系。

#### 3.2 发送消息事务与 Outbox

```go
// Redis INCR 为会话生成单调递增 seq。
seq, err := conversationCache.IncrConvLatestSeq(ctx, conversationID)

err = txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
    // 消息本体是事实数据，失败必须回滚。
    if err := messageRepo.WithTx(tx).Save(ctx, message); err != nil {
        return ErrMessageSave
    }

    // 更新会话最新消息和发送方游标，自己的消息不应成为自己的未读。
    conversationRepo.WithTx(tx).Upsert(ctx, conversation)
    userConversationRepo.WithTx(tx).UpdateReadSeq(ctx, senderConversation)
    userConversationRepo.WithTx(tx).UpdateSyncSeq(ctx, senderConversation)

    // 图片/视频/文件/表情包扩展与主消息同事务。
    if mediaWriter != nil {
        if err := mediaWriter(ctx, tx); err != nil { return err }
    }

    // 不在请求内直接依赖 Kafka，而是保存待发送事件。
    return outboxRepo.WithTx(tx).Create(ctx, outboxEvent)
})
```

为什么这样写：直接执行“写 MySQL -> 发 Kafka”存在双写问题。数据库成功而 Kafka 失败会丢通知；Kafka 成功而数据库回滚会发送幽灵消息。Outbox 与业务数据同事务后，提交结果只有“都存在”或“都不存在”。

`clientMsgId` 在 Redis 中保存短期去重映射。前端因 ACK 超时重发时保持同一个 ID，后端会返回原 `messageId` 而不是再写一条消息。

#### 3.3 已读回执

```go
uconv := userConversationRepo.GetUserConversation(ctx, userID, conversationID)

// 读游标只前进不后退；重复包天然幂等。
if lastReadSeq <= uconv.LastReadSeq {
    return nil
}

uconv.UpdateReadSeq(lastReadSeq)
return txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
    userConversationRepo.WithTx(tx).UpdateReadSeq(ctx, uconv)

    // 最后一条若是自己发的，只更新读位置，不通知自己。
    if senderID == userID { return nil }

    return outboxRepo.WithTx(tx).Create(ctx, readAckEvent)
})
```

解决的问题：刷新后未读数重复出现、重复 ACK 反复写库、自己给自己产生已读通知、读游标更新成功但通知事件丢失。

#### 3.4 历史消息与媒体批量回填

```go
messages := messageRepo.GetHistoryMessage(ctx, conversationID, maxSeq, limit)

// 仓储按倒序高效取最近 N 条，返回前转成正序供 UI 展示。
sort.Slice(messages, func(i, j int) bool { return messages[i].Seq < messages[j].Seq })

// 先按 CType 收集 messageId，再分别批量查四张子表。
images := imageRepo.BatchGetByMessageIDs(ctx, imageIDs)
videos := videoRepo.BatchGetByMessageIDs(ctx, videoIDs)
files := fileRepo.BatchGetByMessageIDs(ctx, fileIDs)
stickers := stickerRepo.BatchGetByMessageIDs(ctx, stickerIDs)
```

为什么批量查询：逐条消息查媒体详情会形成 N+1 查询。按类型一次批量读取，把查询数量稳定在常数级。

#### 3.5 离线与未读

离线接口批量读取用户的所有 `UserConversation`、对应 `Conversation` 和每个会话最新消息；未读数使用：

```text
unread = max(conversation.latestSeq - userConversation.lastReadSeq, 0)
```

只对 `LatestSyncSeq` 落后的会话产生同步项，避免每次重连都更新全部会话。

### 测试数据应用

| 文件 | 作用 |
|---|---|
| `application/testdata/bootstrap.go` | 开发环境重建演示用户、好友、房间、消息和缓存数据 |

它由 `/api/v1/dev` 路由调用，只在 development 环境注册。这样演示环境可以快速得到一致数据，但生产环境不会暴露破坏性入口。

## 4. Ports 文件索引

Ports 是应用层定义的“我需要什么能力”，基础设施层负责“具体怎么实现”。

### 通用端口

| 文件 | 作用 |
|---|---|
| `application/ports/service/auth.go` | 签发 Token 的服务接口 |
| `application/ports/storage/object/object.go` | 普通上传、Multipart、签名 URL 的对象存储接口 |
| `application/ports/mq/taskManager.go` | 普通消息、已读、同步序列事件发布接口 |
| `application/ports/persistence/tx_manager/tx_manager.go` | 事务闭包接口 |

### Repository 端口

| 文件 | 作用 |
|---|---|
| `ports/persistence/repository/user/user.go` | 用户查重、按 ID/手机号查询、批量查询和更新 |
| `ports/persistence/repository/friend/friend.go` | 好友关系查询、列表和创建 |
| `ports/persistence/repository/friend_request/friend_request.go` | 好友申请查询、创建、条件状态更新 |
| `ports/persistence/repository/room/room.go` | 房间创建、状态查询、版本更新 |
| `ports/persistence/repository/room/room_user.go` | 成员加入、关系查询、活跃成员列表 |
| `ports/persistence/repository/file/file.go` | 文件元数据保存与查询 |
| `ports/persistence/repository/message/message.go` | 消息与四种媒体子表接口 |
| `ports/persistence/repository/message/conversation.go` | 会话创建、Upsert、批量查询 |
| `ports/persistence/repository/message/user_conversation.go` | 用户读/同步游标及批量更新 |
| `ports/persistence/repository/message/message_outbox.go` | Outbox 创建、抢占、成功和失败标记 |

### Cache 端口

| 文件 | 作用 |
|---|---|
| `ports/persistence/cache/auth/auth.go` | Access/Refresh Token 缓存 |
| `ports/persistence/cache/conversation/entry.go` | 会话 seq、成员版本、消息去重缓存 |
| `ports/persistence/cache/file/file.go` | 文件缓存、hash 秒传、Multipart 状态和 Part 列表 |
| `ports/persistence/cache/room/room.go` | 房间邀请码映射 |
| `ports/persistence/cache/friend/friend.go` | 好友缓存接口，当前业务使用较少 |
| `ports/persistence/cache/message/message.go` | 消息缓存接口预留，当前主链路未依赖 |

典型端口代码：

```go
type MessageRepository interface {
    Save(ctx context.Context, message *entity.Message) error
    GetHistoryMessage(ctx context.Context, conversationID string, maxSeq int64, limit int) ([]*entity.Message, error)
    GetLatestMessagesByConversationIDs(ctx context.Context, ids []string) ([]*entity.Message, error)
    // 端口当前用 any 避免在 Repository 接口中固定具体事务类型。
    WithTx(tx any) MessageRepository
}
```

为什么每个仓储都有 `WithTx`：应用层拿到同一个 GORM 事务后，可以派生多个绑定到该事务的仓储，实现跨表原子操作，同时不让业务方法直接拼 SQL。当前 `TxManager` 的闭包仍显式使用 `*gorm.DB`，而 Repository 的 `WithTx` 接受 `any`；这完成了基本隔离，但类型安全仍可继续改进。

解决的问题：事务只覆盖一部分仓储、单元测试必须启动真实数据库、业务层被 GORM API 污染。

## 5. Application 错误与 DTO 为什么独立成文件

每个应用模块都有 `dto.go` 与 `errors.go`：

- DTO 是层与层之间的契约，不直接暴露 GORM Model，防止数据库字段变化穿透到前端。
- Application Error 是稳定语义，Transport 可以用 `errors.Is` 映射为 400/401/404/409。
- Domain Error 描述规则失败，Application 负责将其转换为当前用例的错误语义。

这也是之前出现“Entity errors 和 Application errors 不是同一个值，Handler 总命中 default”的原因。正确做法是在应用层使用 `errors.Is` 显式转换，或让应用错误包装领域错误，而不是依赖错误字符串相同。

## 6. 业务单元测试文件

| 文件 | 验证内容 |
|---|---|
| `application/message/message_test.go` | 发消息落库与 Outbox、媒体子表、已读游标与事件、历史游标分页、未读计算、重连同步 seq |
| `application/file/file_test.go` | 普通上传、空文件拒绝、缓存命中刷新 URL、缓存未命中回源 Repository |

测试通过 Fake Repository、Fake Cache、Fake Storage 记录应用服务调用，不启动 MySQL/Kafka/MinIO。典型结构是：

```go
func TestHandleMessageStoresMessageAndCreatesOutbox(t *testing.T) {
    // Arrange：准备 Fake 依赖和输入。
    app := newMessageApplicationWithFakes(...)

    // Act：执行一个完整业务用例，而不是测试私有辅助函数。
    result, err := app.HandleMessage(ctx, dto)

    // Assert：同时验证返回值和持久化副作用。
    require.NoError(t, err)
    require.Len(t, fakeMessageRepo.saved, 1)
    require.Len(t, fakeOutboxRepo.created, 1)
    require.Equal(t, result.MessageId, fakeMessageRepo.saved[0].MessageId)
}
```

这样测试的是业务决策和事务内应该发生的动作。它速度快、失败定位清楚，但不能替代真实 MySQL 锁、Redis 原子命令、Kafka Consumer Group 的集成测试。
