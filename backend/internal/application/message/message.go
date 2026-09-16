package message

import (
	"IM_backend/configs"
	idport "IM_backend/internal/application/ports/id"
	outboxport "IM_backend/internal/application/ports/outbox"
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	friendcache "IM_backend/internal/application/ports/persistence/cache/friend"
	messagecache "IM_backend/internal/application/ports/persistence/cache/message"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	usercach "IM_backend/internal/application/ports/persistence/cache/user"
	conversationrepo "IM_backend/internal/application/ports/persistence/repository/conversation"
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	objectstorage "IM_backend/internal/application/ports/storage/object"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	conversationvo "IM_backend/internal/domain/conversation/value_object"
	fileentity "IM_backend/internal/domain/file/entity"
	friendvo "IM_backend/internal/domain/friend/value_object"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	userentity "IM_backend/internal/domain/user/entity"
	"unicode/utf8"

	"IM_backend/internal/shared/protocol"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

type MessageApplication struct {
	friendCache                  friendcache.FriendCache
	messageCache                 messagecache.MessageCache
	roomMemberCache              roomcache.RoomMemberCache
	roomCache                    roomcache.RoomCache
	messageRepository            messagerepo.MessageRepository
	txManager                    txmanager.TxManager
	userCache                    usercach.UserCache
	userConversationRepository   conversationrepo.UserConversationRepository
	conversationRepository       conversationrepo.ConversationRepository
	friendRepository             friendrepo.FriendRepository
	fileRepository               filerepo.FileRepository
	fileCache                    filecache.FileCache
	objectStorage                objectstorage.ObjectStorage
	messageStickerRepository     messagerepo.MessageStickerRepository
	messageAttachmentsRepository messagerepo.MessageAttachmentsRepository
	messageOutboxRepository      outboxport.Repository
	userRepository               userrepo.UserRepository
	roomUserRepository           roomrepo.RoomUserRepository
	roomRepository               roomrepo.RoomRepository
	sf                           singleflight.Group
	idGenerator                  idport.Generator
	config                       configs.Config
}

func NewMessageApplication(
	friendCache friendcache.FriendCache,
	messageCache messagecache.MessageCache,
	roomMemberCache roomcache.RoomMemberCache,
	roomCache roomcache.RoomCache,
	userCache usercach.UserCache,
	txManager txmanager.TxManager,
	userConversationRepository conversationrepo.UserConversationRepository,
	conversationRepository conversationrepo.ConversationRepository,
	messageOutboxRepository outboxport.Repository,
	friendRepository friendrepo.FriendRepository,
	fileRepository filerepo.FileRepository,
	fileCache filecache.FileCache,
	objectStorage objectstorage.ObjectStorage,
	messageStickerRepository messagerepo.MessageStickerRepository,
	userRepository userrepo.UserRepository,
	messageRepository messagerepo.MessageRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	roomRepository roomrepo.RoomRepository,
	idGenerator idport.Generator,
	config configs.Config,
	messageAttachmentsRepositories ...messagerepo.MessageAttachmentsRepository,

) *MessageApplication {
	app := &MessageApplication{
		friendCache:                friendCache,
		messageCache:               messageCache,
		roomMemberCache:            roomMemberCache,
		roomCache:                  roomCache,
		userCache:                  userCache,
		txManager:                  txManager,
		messageOutboxRepository:    messageOutboxRepository,
		messageRepository:          messageRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
		userRepository:             userRepository,
		roomRepository:             roomRepository,
		roomUserRepository:         roomUserRepository,
		friendRepository:           friendRepository,
		fileRepository:             fileRepository,
		fileCache:                  fileCache,
		objectStorage:              objectStorage,
		messageStickerRepository:   messageStickerRepository,
		idGenerator:                idGenerator,
		config:                     config,
	}
	if len(messageAttachmentsRepositories) > 0 {
		app.messageAttachmentsRepository = messageAttachmentsRepositories[0]
	}
	return app
}

func (ma *MessageApplication) HandleReadMessage(
	ctx context.Context,
	userId string,
	messageId string,
) error {
	message, err := ma.messageRepository.FindByMessageID(ctx, messageId)
	if err != nil {
		return err
	}
	if message == nil {
		return ErrMessageNotFound
	}

	conversationId := message.ConversationId
	lastReadSeq := message.Seq
	if lastReadSeq <= 0 {
		return ErrMessageSeq
	}
	if ma.txManager == nil || ma.messageOutboxRepository == nil {
		return fmt.Errorf("消息出箱组件未配置")
	}

	uconv, err := ma.userConversationRepository.GetUserConversation(ctx, userId, conversationId)
	if err != nil {
		return ErrConversationNotFound
	}

	if uconv == nil {
		return ErrUserConversationNotFound
	}

	conv, err := ma.conversationRepository.GetByID(ctx, conversationId)
	if err != nil {
		return err
	}

	if conv == nil {
		return ErrConversationNotFound
	}
	canAccess, err := ma.canAccessConversation(ctx, conv, userId)
	if err != nil {
		return err
	}
	if !canAccess {
		return ErrForbidden
	}

	// 当前只为私聊维护对端已读水位，群聊已读语义单独处理。
	if conv.Convtype != conversationvo.PrivateChat {
		return nil
	}

	if lastReadSeq > conv.LatestSeq {
		return ErrMessageSeq
	}

	var peerUserId string
	switch userId {
	case conv.UserId1:
		peerUserId = conv.UserId2
	case conv.UserId2:
		peerUserId = conv.UserId1
	default:
		return ErrForbidden
	}

	if peerUserId == "" || peerUserId == userId {
		return ErrForbidden
	}

	peerProfiles, err := ma.loadReadUserProfiles(ctx, []string{peerUserId})
	if err != nil {
		return err
	}
	if peerProfiles[peerUserId] == nil {
		return ErrUserNotFonund
	}
	if lastReadSeq <= uconv.LastReadSeq {
		return nil
	}

	uconv.UpdateReadSeq(lastReadSeq)
	return ma.txManager.WithinTransaction(ctx, func(tx any) error {
		advanced, err := ma.userConversationRepository.WithTx(tx).AdvanceReadSeq(ctx, uconv)
		if err != nil {
			return err
		}
		if !advanced {
			return nil
		}

		readEvent := protocol.MessageReadCommittedEvent{
			ReaderId:       userId,
			ConversationId: conversationId,
			LastReadSeq:    lastReadSeq,
			ConvType:       protocol.ConvType(conv.Convtype),
			// 私聊目前只推送已读水位。群聊已读通知应根据 ReaderId 填阅读者头像，
			// 不能把本次查询的对端头像误当成阅读者头像。
		}

		eventPayload, err := json.Marshal(readEvent)
		if err != nil {
			return err
		}

		payload, err := json.Marshal(protocol.Envelope{
			To:      peerUserId,
			Payload: eventPayload,
		})

		if err != nil {
			return err
		}

		outbox := &outboxport.Entry{
			EventType:  string(protocol.EventReadMessageCommitted),
			MessageKey: userId + ":" + conversationId,
			Payload:    payload,
		}
		return ma.messageOutboxRepository.WithTx(tx).Create(ctx, outbox)
	})
}

func (ma *MessageApplication) normalizeMediaDTO(ctx context.Context, dto *SendMessageDTO) error {
	if dto == nil {
		return nil
	}
	switch messagevo.CType(dto.Type) {
	case messagevo.Image, messagevo.Video, messagevo.File:
		if dto.FileID == "" || ma.fileRepository == nil {
			return fmt.Errorf("文件引用不能为空")
		}
		file, err := ma.findUploadedFile(ctx, dto.FileID, dto.SenderID)
		if err != nil {
			return err
		}
		if file == nil {
			return fmt.Errorf("文件不存在或不可用：%s", dto.FileID)
		}

		// 文件名、大小和 MIME 以服务端文件记录为准；客户端媒体元数据全部忽略。
		dto.FileName = file.FileName
		dto.FileSize = file.Size
		dto.MimeType = file.ContentType
		dto.Content = file.FileName
		dto.Width = 0
		dto.Height = 0
		dto.DurationMs = nil

		switch messagevo.CType(dto.Type) {
		case messagevo.Image:
			if !strings.HasPrefix(strings.ToLower(file.ContentType), "image/") {
				return fmt.Errorf("图片消息引用的文件类型不匹配")
			}
		case messagevo.Video:
			if !strings.HasPrefix(strings.ToLower(file.ContentType), "video/") {
				return fmt.Errorf("视频消息引用的文件类型不匹配")
			}
		}
	case messagevo.Sticker:
		if dto.StickerID == "" {
			return fmt.Errorf("表情内容不能为空")
		}
	}
	return nil
}

// findUploadedFile 优先读取按 fileId 预热的文件缓存，缓存未命中或不满足
// 当前用户归属校验时再回源数据库。事务内仍会通过 FOR UPDATE 再次确认，
// 防止文件清理与发送消息并发导致引用到已删除文件。
func (ma *MessageApplication) findUploadedFile(ctx context.Context, fileID, uploaderID string) (*fileentity.File, error) {
	if ma.fileCache != nil {
		cached, cacheErr := ma.fileCache.GetFileMetadata(ctx, fileID)
		if cacheErr != nil {
			log.Printf("读取文件元数据缓存失败: fileId=%s err=%v", fileID, cacheErr)
		} else if cached != nil &&
			cached.Status == fileentity.FileStatusUploaded &&
			cached.UploaderId == uploaderID {
			return cached, nil
		}
	}

	if ma.fileRepository == nil {
		return nil, errors.New("文件仓储未配置")
	}
	file, err := ma.fileRepository.FindUploadedFileByIDAndUploader(ctx, fileID, uploaderID)
	if err != nil || file == nil {
		return file, err
	}
	if ma.fileCache != nil {
		_ = ma.fileCache.SetFileMetadata(ctx, file, ma.fileMetadataCacheTTL())
	}
	return file, nil
}

func (ma *MessageApplication) fileMetadataCacheTTL() time.Duration {
	if ma.config.Storage.MinIO.CacheTTLSeconds > 0 {
		return time.Duration(ma.config.Storage.MinIO.CacheTTLSeconds) * time.Second
	}
	return 24 * time.Hour
}

func (ma *MessageApplication) buildAttachment(dto *SendMessageDTO, messageId string) (*messageentity.MessageAttachment, error) {
	if ma.messageAttachmentsRepository == nil {
		return nil, fmt.Errorf("消息附件仓储未配置")
	}
	attachmentId, err := ma.idGenerator.Generate()
	if err != nil {
		return nil, err
	}
	dto.AttachmentID = attachmentId
	attachmentTTL := ma.config.Message.AttachmentTTLSeconds
	if attachmentTTL <= 0 {
		return nil, errors.New("消息附件有效期配置无效")
	}
	now := time.Now().UnixMilli()
	return messageentity.NewMessageAttachment(
		attachmentId,
		messageId,
		dto.ConversationID,
		dto.FileID,
		int8(dto.Type),
		now+attachmentTTL*int64(time.Second/time.Millisecond),
		now,
	), nil
}

func (ma *MessageApplication) buildMediaWriter(dto *SendMessageDTO, messageId string) (func(context.Context, any) error, error) {
	if dto == nil {
		return nil, nil
	}

	switch messagevo.CType(dto.Type) {
	case messagevo.Image, messagevo.File, messagevo.Video:
		if dto.FileID == "" {
			return nil, fmt.Errorf("媒体文件不能为空")
		}
		attachment, err := ma.buildAttachment(dto, messageId)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, tx any) error {
			return ma.messageAttachmentsRepository.WithTx(tx).Create(ctx, attachment)
		}, nil
	case messagevo.Sticker:
		if ma.messageStickerRepository == nil {
			return nil, fmt.Errorf("消息表情仓储未配置")
		}
		if dto.StickerID == "" {
			return nil, fmt.Errorf("表情内容不能为空")
		}
		item := messageentity.NewMessageSticker(
			messageId,
			dto.StickerID,
			dto.PackID,
			dto.Width,
			dto.Height,
		)
		return func(ctx context.Context, tx any) error {
			return ma.messageStickerRepository.WithTx(tx).Create(ctx, item)
		}, nil
	default:
		return nil, nil
	}
}

func (ma *MessageApplication) isRoomConvMember(ctx context.Context, userID, roomID string) error {
	key := "room_member:" + roomID + ":" + userID
	// 使用 sf 来减少房间内成员频繁发送消息时造成缓存的频繁查询
	resultCh := ma.sf.DoChan(key, func() (any, error) {
		ctx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			3*time.Second,
		)
		defer cancel()

		var state *roomcache.MemberState
		var hit bool
		var cacheErr error
		if ma.roomMemberCache != nil {
			state, hit, cacheErr = ma.roomMemberCache.GetMember(ctx, roomID, userID)
		}
		if cacheErr == nil && hit {
			if state == nil || state.Status == roomvo.Left {
				return nil, ErrNotRoomMember
			}
			if state.Status == roomvo.BeKicked {
				return nil, ErrForbidden
			}
			if state.Status != roomvo.Activate && state.Status != roomvo.BeMuted {
				return nil, ErrNotRoomMember
			}
			return state, nil
		}

		member, err := ma.roomUserRepository.GetRelationByIDs(userID, roomID)
		if err != nil {
			if errors.Is(err, roomentity.ErrMemberNotFound) {
				return nil, ErrNotRoomMember
			}
			return nil, err
		}
		if member.Status != roomvo.Activate && member.Status != roomvo.BeMuted {
			if ma.roomMemberCache != nil {
				_, _ = ma.roomMemberCache.SetMemberIfVersionGreater(ctx, roomID, userID, &roomcache.MemberState{
					Status: member.Status, Role: member.Role,
					MuteUntil: member.MuteUtil, Version: member.Version,
				})
			}
			return nil, ErrNotRoomMember
		}

		state = &roomcache.MemberState{
			Status:    member.Status,
			Role:      member.Role,
			MuteUntil: member.MuteUtil,
			Version:   member.Version,
		}
		if ma.roomMemberCache != nil {
			_, _ = ma.roomMemberCache.SetMemberIfVersionGreater(ctx, roomID, userID, state)
		}
		return state, nil
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return result.Err
		}

		state, ok := result.Val.(*roomcache.MemberState)
		if !ok || state == nil {
			return ErrNotRoomMember
		}

		if state.Status == roomvo.BeKicked {
			return ErrForbidden
		}
		if state.Status == roomvo.BeMuted && (state.MuteUntil == nil || *state.MuteUntil > time.Now().UnixMilli()) {
			return ErrForbidden
		}
	}

	return nil
}

func (ma *MessageApplication) isPrivateConvMember(ctx context.Context, userID, recvID string) error {
	key := "friend:" + userID + ":" + recvID
	resultCh := ma.sf.DoChan(key, func() (any, error) {
		ctx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			2*time.Second,
		)
		defer cancel()

		state, hit, cacheErr := ma.friendCache.GetRelation(ctx, userID, recvID)
		if cacheErr == nil && hit {
			if state == nil || state.Status != friendvo.Friend {
				return nil, ErrNotFriends
			}
			return state, nil
		}

		relation, err := ma.friendRepository.FindRelation(userID, recvID)
		if err != nil {
			return nil, err
		}
		if relation == nil || relation.Status != friendvo.Friend {
			_ = ma.friendCache.SetRelationNotFound(ctx, userID, recvID)
			return nil, ErrNotFriends
		}

		state = &friendcache.RelationState{Status: relation.Status}
		_ = ma.friendCache.SetRelation(ctx, userID, recvID, state)
		return state, nil
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-resultCh:
		state, ok := result.Val.(*friendcache.RelationState)
		if !ok || state == nil {
			return ErrNotFriends
		}

		if state.Status != friendvo.Friend {
			return ErrForbidden
		}
	}
	return nil
}

func (ma *MessageApplication) checkConvMember(ctx context.Context, dto SendMessageDTO) error {
	switch conversationvo.ConvType(dto.ConversationType) {
	case conversationvo.PrivateChat:
		return ma.isPrivateConvMember(ctx, dto.SenderID, dto.ReceiverID)
	case conversationvo.RoomChat:
		return ma.isRoomConvMember(ctx, dto.SenderID, dto.ReceiverID)
	default:
		return ErrConversationNotFound
	}
}

func (ma *MessageApplication) validateFileReference(dto SendMessageDTO) error {
	if dto.FileID == "" {
		return errors.New("文件 ID 不能为空")
	}

	if len(dto.FileID) > ma.config.Message.MaxIdentifierLength {
		return errors.New("文件 ID 无效")
	}

	return nil
}

// 根据消息类型检验消息内容
func (ma *MessageApplication) validateMessageCType(dto SendMessageDTO) error {
	if dto.SenderID == "" || dto.ReceiverID == "" {
		return errors.New("发送方或接收方不能为空")
	}

	switch messagevo.CType(dto.Type) {
	case messagevo.Text:
		content := strings.TrimSpace(dto.Content)
		if content == "" {
			return errors.New("文本内容不能为空")
		}

		if utf8.RuneCountInString(content) > ma.config.Message.MaxTextRunes {
			return errors.New("文本内容过长")
		}

		// 文本消息，其他的媒体消息字段都为空
		if dto.FileID != "" ||
			dto.StickerID != "" ||
			dto.PackID != "" ||
			dto.Width != 0 ||
			dto.Height != 0 ||
			dto.DurationMs != nil {
			return errors.New("文本消息包含非法媒体字段")
		}
	case messagevo.Image:
		if err := ma.validateFileReference(dto); err != nil {
			return err
		}

		if dto.StickerID != "" ||
			dto.PackID != "" {
			return errors.New("图片消息包含非法字段")
		}
	case messagevo.Video:
		if err := ma.validateFileReference(dto); err != nil {
			return err
		}

		if dto.StickerID != "" || dto.PackID != "" {
			return errors.New("视频消息包含非法字段")
		}
	case messagevo.File:
		if err := ma.validateFileReference(dto); err != nil {
			return err
		}

		if dto.StickerID != "" ||
			dto.PackID != "" {
			return errors.New("文件消息包含非法字段")
		}

	case messagevo.Sticker:
		if dto.StickerID == "" {
			return errors.New("表情 ID 不能为空")
		}
		if len(dto.StickerID) > ma.config.Message.MaxIdentifierLength || len(dto.PackID) > ma.config.Message.MaxIdentifierLength {
			return errors.New("表情标识过长")
		}
	}

	return nil
}

// HandleSendMessage 校验并持久化一条消息，并在同一事务中创建 Outbox 事件。
// 消息、会话序号、附件关联和 Outbox 成功提交后，调用方才会收到成功 ACK。
func (ma *MessageApplication) HandleSendMessage(ctx context.Context, dto SendMessageDTO) (*MessageAckDTO, error) {
	conversationId := conversationentity.GetConversationID(dto.SenderID, dto.ReceiverID, dto.ConversationType)
	dto.ConversationID = conversationId

	if err := ma.validateMessageCType(dto); err != nil {
		return &MessageAckDTO{
			ClientMessageID: dto.ClientMessageID,
			Status:          string(protocol.AckStatusFailed),
		}, err
	}

	requestHash, err := buildMessageRequestHash(dto)
	if err != nil {
		return &MessageAckDTO{
			ClientMessageID: dto.ClientMessageID,
			Status:          string(protocol.AckStatusFailed),
		}, err
	}

	// 幂等检查：同一 clientMsgId 在 5 分钟内只处理一次，当遇到一样的消息时，不会重复入库，而是直接返回之前的结果
	if dto.ClientMessageID != "" && ma.messageCache != nil {
		if entry, err := ma.messageCache.GetDedupEntry(ctx, dto.SenderID, dto.ClientMessageID); err == nil && entry.MessageID != "" {
			if entry.RequestHash != requestHash {
				return &MessageAckDTO{ClientMessageID: dto.ClientMessageID, Status: string(protocol.AckStatusFailed)}, messageentity.ErrClientMessageConflict
			}

			// 兼容只保存了 messageId/requestHash 的旧缓存记录。新记录会直接
			// 从 Redis 恢复 ACK 所需的完整元数据，旧记录则尝试从数据库补齐。
			needsRecovery := entry.ConversationID == "" || entry.Seq == 0
			if needsRecovery && ma.messageRepository != nil {
				if existing, findErr := ma.messageRepository.FindByClientMsgID(ctx, dto.SenderID, dto.ClientMessageID); findErr == nil && existing != nil {
					if recovered := ma.existingMessageResult(ctx, dto, existing); recovered != nil {
						return recovered, nil
					}
				}
			}

			conversationID := entry.ConversationID
			if conversationID == "" {
				conversationID = conversationId
			}
			return &MessageAckDTO{
				ClientMessageID: dto.ClientMessageID,
				ConversationID:  conversationID,
				MessageID:       entry.MessageID,
				Seq:             entry.Seq,
				SendTime:        entry.SendTime,
				Status:          string(protocol.AckStatusSent),
				AttachmentID:    entry.AttachmentID,
			}, nil
		}
	}

	if err := ma.checkConvMember(ctx, dto); err != nil {
		return &MessageAckDTO{
			ClientMessageID: dto.ClientMessageID,
			Status:          string(protocol.AckStatusFailed),
		}, err
	}

	if err := ma.normalizeMediaDTO(ctx, &dto); err != nil {
		return &MessageAckDTO{
			ClientMessageID: dto.ClientMessageID,
			Status:          string(protocol.AckStatusFailed),
		}, err
	}

	messageId, err := ma.idGenerator.Generate()
	if err != nil {
		return &MessageAckDTO{
			ClientMessageID: dto.ClientMessageID,
			Status:          string(protocol.AckStatusFailed),
		}, err
	}

	messageType := messagevo.CType(dto.Type)
	message := messageentity.NewMessage(
		messageId,
		conversationId,
		dto.SenderID,
		0,
		messageType,
		dto.Content,
		dto.VideoID,
		dto.VideoTime,
	)
	if dto.ClientMessageID != "" {
		clientMsgID := dto.ClientMessageID
		message.ClientMsgId = &clientMsgID
	}
	message.RequestHash = requestHash

	mediaWriter, err := ma.buildMediaWriter(&dto, messageId)
	if err != nil {
		return &MessageAckDTO{
			ClientMessageID: dto.ClientMessageID,
			MessageID:       messageId,
			Status:          string(protocol.AckStatusFailed),
		}, err
	}

	conv := conversationentity.NewConversation(
		conversationId,
		dto.SenderID,
		dto.ReceiverID,
		dto.ConversationType,
		0,
		messageId,
	)

	userConv := conversationentity.BuildUserConversation(
		dto.SenderID,
		conversationId,
		0,
	)

	// 弹幕需要同时满足前端发送有视频时间以及在房间内
	isDanmaku := dto.ConversationType == int(conversationvo.RoomChat) && dto.VideoTime != nil
	if ma.txManager == nil || ma.messageOutboxRepository == nil {
		return nil, fmt.Errorf("消息出箱组件未配置")
	}

	value := ctx.Value("op")
	op, ok := value.(string)
	if !ok || op == "" {
		return nil, ErrUnknown
	}

	senderUsername := ma.getUsername(dto.SenderID)
	messageEvent := protocol.MessageEvent{
		MessageId:      messageId,
		ConversationId: conversationId,
		SenderId:       dto.SenderID,
		SenderUsername: senderUsername,
		RecvId:         dto.ReceiverID,
		ConvType:       protocol.ConvType(dto.ConversationType),
		CType:          dto.Type,
		Content:        dto.Content,
		VideoId:        dto.VideoID,
		VideoTime:      dto.VideoTime,
		SendTime:       message.SendTime,
		ClientMsgId:    dto.ClientMessageID,
		Status:         int8(message.Status),
		AttachmentId:   dto.AttachmentID,
		Width:          dto.Width,
		Height:         dto.Height,
		DurationMs:     dto.DurationMs,
		StickerId:      dto.StickerID,
		PackId:         dto.PackID,
		HasVideoTime:   dto.VideoTime != nil,
	}

	var nextSeq int64 = 0
	err = ma.txManager.WithinTransaction(ctx, func(tx any) error {
		msgRepo := ma.messageRepository.WithTx(tx)
		convRepo := ma.conversationRepository.WithTx(tx)
		userConvRepo := ma.userConversationRepository.WithTx(tx)
		outboxRepo := ma.messageOutboxRepository.WithTx(tx)
		var mediaFile *fileentity.File

		if messagevo.CType(dto.Type) == messagevo.Image || messagevo.CType(dto.Type) == messagevo.Video || messagevo.CType(dto.Type) == messagevo.File {
			lockedFile, lockErr := ma.fileRepository.WithTx(tx).FindUploadedFileByIDAndUploaderForUpdate(ctx, dto.FileID, dto.SenderID)
			if lockErr != nil {
				return lockErr
			}
			if lockedFile == nil {
				return fmt.Errorf("file is missing or being deleted: %s", dto.FileID)
			}
			dto.FileName = lockedFile.FileName
			dto.FileSize = lockedFile.Size
			dto.MimeType = lockedFile.ContentType
			dto.Content = lockedFile.FileName
			message.Content = dto.Content
			messageEvent.Content = dto.Content
			mediaFile = lockedFile
		}

		nextSeq, err = convRepo.UpdateLatestSequence(ctx, conversationId, messageId)
		if err != nil {
			if errors.Is(err, conversationentity.ErrConversationNotCreated) {
				return ErrConversationNotFound
			}
			return ErrConversationSequenceUpdate
		}

		message.Seq = nextSeq
		conv.LatestSeq = nextSeq
		messageEvent.Seq = nextSeq
		userConv.UpdateReadSeq(nextSeq)

		if err := msgRepo.CreateNewMessage(ctx, message); err != nil {
			if errors.Is(err, messageentity.ErrDuplicateClientMessage) {
				return err
			}
			return ErrMessageSave
		}

		if !isDanmaku {
			if err := userConvRepo.UpdateReadSeq(ctx, userConv); err != nil {
				return err
			}
		}

		if mediaWriter != nil {
			if err := mediaWriter(ctx, tx); err != nil {
				return err
			}
		}

		eventPayload, err := json.Marshal(messageEvent)
		if err != nil {
			return err
		}

		messagePayload, err := json.Marshal(protocol.Envelope{
			From:    messageEvent.SenderId,
			To:      messageEvent.RecvId,
			Payload: eventPayload,
		})

		if err != nil {
			return err
		}

		outbox := &outboxport.Entry{
			EventType:  string(protocol.EventTypeSendMessage),
			MessageKey: conversationId,
			Payload:    messagePayload,
		}
		if err := outboxRepo.Create(ctx, outbox); err != nil {
			return err
		}

		if mediaFile != nil && dto.AttachmentID != "" {
			warmupEvent := protocol.FileCardWarmupEvent{
				AttachmentID:       dto.AttachmentID,
				MessageID:          messageId,
				ConversationID:     conversationId,
				FileID:             mediaFile.FileId,
				ObjectKey:          mediaFile.ObjectKey,
				FileName:           mediaFile.FileName,
				ContentType:        mediaFile.ContentType,
				Size:               mediaFile.Size,
				Status:             mediaFile.Status,
				CType:              dto.Type,
				AttachmentExpireAt: time.Now().Add(time.Duration(ma.config.Message.AttachmentTTLSeconds) * time.Second).UnixMilli(),
			}
			warmupPayload, err := json.Marshal(warmupEvent)
			if err != nil {
				return err
			}
			warmupEnvelope, err := json.Marshal(protocol.Envelope{
				From:    messageEvent.SenderId,
				To:      mediaFile.FileId,
				Payload: warmupPayload,
			})
			if err != nil {
				return err
			}
			if err := outboxRepo.Create(ctx, &outboxport.Entry{
				EventType:  string(protocol.EventFileCardWarmup),
				MessageKey: mediaFile.FileId,
				Payload:    warmupEnvelope,
			}); err != nil {
				return err
			}
		}

		return nil
	})

	// 判断是否出发唯一键索引冲突
	if errors.Is(err, messageentity.ErrDuplicateClientMessage) && dto.ClientMessageID != "" {
		// 消息重复写入，尝试从数据库中找到这条消息并直接返回
		existing, findErr := ma.messageRepository.FindByClientMsgID(ctx, dto.SenderID, dto.ClientMessageID)
		if findErr == nil && existing != nil {
			if !sameClientMessage(dto, existing, requestHash) {
				return &MessageAckDTO{
					ClientMessageID: dto.ClientMessageID,
					Status:          string(protocol.AckStatusFailed),
				}, messageentity.ErrClientMessageConflict
			}
			result := ma.existingMessageResult(ctx, dto, existing)
			if result != nil && ma.messageCache != nil {
				_, _ = ma.messageCache.SetDedupEntry(ctx, dto.SenderID, dto.ClientMessageID, messagecache.DedupEntry{
					MessageID:      result.MessageID,
					RequestHash:    requestHash,
					ConversationID: result.ConversationID,
					Seq:            result.Seq,
					AttachmentID:   result.AttachmentID,
					SendTime:       result.SendTime,
				}, 5*time.Minute)
			}
			return result, nil
		}
		if findErr != nil {
			return nil, findErr
		}
	}

	if err != nil {
		return &MessageAckDTO{
			ClientMessageID: dto.ClientMessageID,
			MessageID:       messageId,
			Status:          string(protocol.AckStatusFailed),
		}, err
	}

	// 标记已处理，5 分钟内同一 clientMsgId 幂等返回
	if dto.ClientMessageID != "" && ma.messageCache != nil {
		_, _ = ma.messageCache.SetDedupEntry(ctx, dto.SenderID, dto.ClientMessageID, messagecache.DedupEntry{
			MessageID:      messageId,
			RequestHash:    requestHash,
			ConversationID: conversationId,
			Seq:            nextSeq,
			AttachmentID:   dto.AttachmentID,
			SendTime:       message.SendTime,
			SenderUsername: senderUsername,
		}, 5*time.Minute)
	}

	return &MessageAckDTO{
		ClientMessageID: dto.ClientMessageID,
		ConversationID:  conversationId,
		MessageID:       messageId,
		AttachmentID:    dto.AttachmentID,
		Seq:             nextSeq,
		SendTime:        message.SendTime,
		Status:          string(protocol.AckStatusSent),
	}, nil
}

// existingMessageResult 将数据库中的已有消息转换成幂等请求的返回值。
// ACK 只依赖消息主表和附件关联，不需要查询文件元数据或生成访问 URL。
func (ma *MessageApplication) existingMessageResult(
	ctx context.Context,
	dto SendMessageDTO,
	existing *messageentity.Message,
) *MessageAckDTO {
	if existing == nil {
		return nil
	}

	attachmentID := ""
	if ma.messageAttachmentsRepository != nil {
		attachments, err := ma.messageAttachmentsRepository.BatchGetByMessageIDs(ctx, []string{existing.MessageId}, false)
		if err == nil {
			if attachment := attachments[existing.MessageId]; attachment != nil {
				attachmentID = attachment.AttachmentId
			}
		}
	}

	return &MessageAckDTO{
		ClientMessageID: dto.ClientMessageID,
		ConversationID:  existing.ConversationId,
		MessageID:       existing.MessageId,
		Seq:             existing.Seq,
		AttachmentID:    attachmentID,
		SendTime:        existing.SendTime,
		Status:          string(protocol.AckStatusSent),
	}
}

type messageRequestFingerprint struct {
	ConversationID string `json:"conversationId"`
	CType          int    `json:"cType"`
	Content        string `json:"content"`
	FileID         string `json:"fileId"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	DurationMs     *int64 `json:"durationMs,omitempty"`
	StickerID      string `json:"stickerId"`
	PackID         string `json:"packId"`
	VideoID        string `json:"videoId"`
	VideoTime      *int64 `json:"videoTime,omitempty"`
}

func buildMessageRequestHash(dto SendMessageDTO) (string, error) {
	content := strings.TrimSpace(dto.Content)
	switch messagevo.CType(dto.Type) {
	case messagevo.Image, messagevo.Video, messagevo.File:
		// 媒体消息的身份由 FileId 表示，不依赖文件名等数据库元数据。
		content = ""
		dto.Width = 0
		dto.Height = 0
		dto.DurationMs = nil
	}

	payload, err := json.Marshal(messageRequestFingerprint{
		ConversationID: dto.ConversationID,
		CType:          dto.Type,
		Content:        content,
		FileID:         dto.FileID,
		Width:          dto.Width,
		Height:         dto.Height,
		DurationMs:     dto.DurationMs,
		StickerID:      dto.StickerID,
		PackID:         dto.PackID,
		VideoID:        dto.VideoID,
		VideoTime:      dto.VideoTime,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func sameClientMessage(dto SendMessageDTO, existing *messageentity.Message, requestHash string) bool {
	if existing == nil {
		return false
	}
	if existing.RequestHash != "" {
		return existing.RequestHash == requestHash
	}
	return existing.ConversationId == conversationentity.GetConversationID(dto.SenderID, dto.ReceiverID, dto.ConversationType) &&
		existing.Type == messagevo.CType(dto.Type) &&
		existing.Content == dto.Content &&
		existing.VideoId == dto.VideoID &&
		sameOptionalInt64(existing.VideoTime, dto.VideoTime)
}

func sameOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// 通过游标的方式来获取历史记录
func (ma *MessageApplication) GetHistoryMessages(
	ctx context.Context,
	conversationId string,
	userId string,
	limit int,
	cursor int64,
) ([]MessageDTO, int64, bool, error) {
	limit = ma.normalizeHistoryLimit(limit)

	resolvedConversationID, err := ma.resolveHistoryConversationID(ctx, conversationId, userId)
	if err != nil {
		return nil, -1, false, err
	}
	conversationId = resolvedConversationID

	var maxSeq int64
	switch cursor {
	case 0:
		// 表示从最新的消息开始读取
		maxSeq = math.MaxInt64
	case -1:
		// 没有消息了
		return []MessageDTO{}, -1, false, nil
	default:
		maxSeq = cursor
	}

	msgs, err := ma.messageRepository.GetHistoryMessage(
		ctx,
		conversationId,
		maxSeq,
		limit+1,
	)
	if err != nil {
		return nil, -1, false, err
	}

	var nextCursor int64 = -1
	var hasMore bool = len(msgs) > limit

	if hasMore {
		msgs = msgs[:limit]
		nextCursor = msgs[len(msgs)-1].Seq
	}

	msgsApp := toMessageDTOs(msgs)
	if err := ma.fillAttachmentIDs(ctx, msgsApp); err != nil {
		return nil, -1, false, err
	}

	sort.Slice(msgsApp, func(i, j int) bool {
		return msgsApp[i].Seq < msgsApp[j].Seq
	})

	return msgsApp, nextCursor, hasMore, nil
}

// GetMessagesBySeqs 按指定 seq 批量读取消息，用于修复客户端本地消息索引中的缺口。
// 它不是增量同步：seq 可能是离散的，查询结果只返回数据库中实际存在的消息。
func (ma *MessageApplication) GetMessagesBySeqs(
	ctx context.Context,
	conversationId string,
	userId string,
	seqs []int64,
) ([]MessageDTO, error) {
	if len(seqs) == 0 {
		return []MessageDTO{}, nil
	}
	if len(seqs) > 500 {
		return nil, ErrTooManySeqs
	}

	resolvedConversationID, _, err := ma.resolveHistoryConversation(ctx, conversationId, userId)
	if err != nil {
		return nil, err
	}

	uniqueSeqs := make([]int64, 0, len(seqs))
	seen := make(map[int64]struct{}, len(seqs))
	for _, seq := range seqs {
		if seq <= 0 {
			continue
		}
		if _, ok := seen[seq]; ok {
			continue
		}
		seen[seq] = struct{}{}
		uniqueSeqs = append(uniqueSeqs, seq)
	}
	if len(uniqueSeqs) == 0 {
		return []MessageDTO{}, nil
	}

	msgs, err := ma.messageRepository.ListBySeqs(ctx, resolvedConversationID, uniqueSeqs)
	if err != nil {
		return nil, err
	}

	msgsApp := toMessageDTOs(msgs)
	if err := ma.fillAttachmentIDs(ctx, msgsApp); err != nil {
		return nil, err
	}
	sort.SliceStable(msgsApp, func(i, j int) bool {
		return msgsApp[i].Seq < msgsApp[j].Seq
	})
	return msgsApp, nil
}

func (ma *MessageApplication) SyncMessages(
	ctx context.Context,
	conversationId string,
	userId string,
	afterSeq int64,
) ([]MessageDTO, error) {
	if afterSeq < 0 {
		afterSeq = 0
	}

	resolvedConversationID, conversation, err := ma.resolveHistoryConversation(ctx, conversationId, userId)
	if err != nil {
		return nil, err
	}
	latestSeq := conversation.LatestSeq
	if latestSeq <= afterSeq {
		return []MessageDTO{}, nil
	}

	// 大群消息消费后已经写入 Redis 的 seq ZSET，优先从缓存同步。Redis 只是加速层，缓存未覆盖完整区间或读取失败时继续回源 MySQL。
	if conversation.Convtype == conversationvo.RoomChat && ma.roomCache != nil {
		page, cacheErr := ma.roomCache.ListMessageAfterSeq(
			ctx,
			resolvedConversationID,
			afterSeq,
			latestSeq,
		)
		if cacheErr != nil {
			log.Printf(
				"读取房间近期消息缓存失败: conversation=%s afterSeq=%d err=%v",
				resolvedConversationID,
				afterSeq,
				cacheErr,
			)
		} else if page.Covered {
			return messageEventsToAppDTO(page.Events, resolvedConversationID), nil
		}
	}

	syncKey := fmt.Sprintf(
		"message-sync-all:%s:%d:%d",
		resolvedConversationID,
		afterSeq,
		latestSeq,
	)

	result, err, _ := ma.sf.Do(syncKey, func() (any, error) {
		msgs, err := ma.messageRepository.ListAfterSeq(
			ctx,
			resolvedConversationID,
			afterSeq,
		)
		if err != nil {
			return nil, err
		}

		filtered := make([]*messageentity.Message, 0, len(msgs))
		for _, msg := range msgs {
			if msg == nil || msg.Seq > latestSeq {
				continue
			}
			filtered = append(filtered, msg)
		}
		msgs = filtered

		msgsApp := toMessageDTOs(msgs)
		if err := ma.fillAttachmentIDs(ctx, msgsApp); err != nil {
			return nil, err
		}
		sort.SliceStable(msgsApp, func(i, j int) bool {
			return msgsApp[i].Seq < msgsApp[j].Seq
		})

		if conversation.Convtype == conversationvo.RoomChat && ma.roomCache != nil {
			events := messageAppDTOsToEvents(msgsApp, resolvedConversationID)
			if err := ma.roomCache.WarmRecentMessageEvents(ctx, resolvedConversationID, events); err != nil {
				log.Printf(
					"回填房间近期消息缓存失败: conversation=%s afterSeq=%d latestSeq=%d err=%v",
					resolvedConversationID,
					afterSeq,
					latestSeq,
					err,
				)
			}
		}

		return msgsApp, nil
	})
	if err != nil {
		return nil, err
	}

	syncResult, ok := result.([]MessageDTO)
	if !ok {
		return nil, errors.New("同步消息结果类型错误")
	}

	return syncResult, nil
}

// messageEventsToAppDTO 将近期消息缓存中的内部事件转换为应用层消息 DTO。
// 转换时补齐会话 ID、状态和附件 ID，供历史或同步接口复用统一的消息响应链路。
func messageEventsToAppDTO(events []protocol.MessageEvent, conversationID string) []MessageDTO {
	if len(events) == 0 {
		return []MessageDTO{}
	}

	result := make([]MessageDTO, 0, len(events))
	for _, event := range events {
		if event.MessageId == "" || event.Seq <= 0 {
			continue
		}

		resolvedID := event.ConversationId
		if resolvedID == "" {
			resolvedID = conversationID
		}

		item := MessageDTO{
			SenderID:       event.SenderId,
			ConversationID: resolvedID,
			MessageID:      event.MessageId,
			Seq:            event.Seq,
			Type:           event.CType,
			Content:        event.Content,
			VideoID:        event.VideoId,
			SendTime:       event.SendTime,
			Status:         event.Status,
			AttachmentID:   event.AttachmentId,
			Width:          event.Width,
			Height:         event.Height,
			DurationMs:     event.DurationMs,
			StickerID:      event.StickerId,
			PackID:         event.PackId,
			VideoTime:      event.VideoTime,
		}
		if item.Status == 0 {
			item.Status = int8(messagevo.Normal)
		}
		if event.ClientMsgId != "" {
			clientMsgID := event.ClientMsgId
			item.ClientMessageID = &clientMsgID
		}
		result = append(result, item)
	}

	return result
}

// messageAppDTOsToEvents 将应用层消息 DTO 转换为内部事件，用于回填房间近期消息缓存。
// 内部事件可以携带后端投递所需的路由信息，但这些字段会在 WebSocket 编码时被过滤。
func messageAppDTOsToEvents(messages []MessageDTO, conversationID string) []protocol.MessageEvent {
	if len(messages) == 0 {
		return []protocol.MessageEvent{}
	}

	events := make([]protocol.MessageEvent, 0, len(messages))
	for _, message := range messages {
		if message.MessageID == "" || message.Seq <= 0 {
			continue
		}

		resolvedID := message.ConversationID
		if resolvedID == "" {
			resolvedID = conversationID
		}

		clientMsgID := ""
		if message.ClientMessageID != nil {
			clientMsgID = *message.ClientMessageID
		}

		events = append(events, protocol.MessageEvent{
			MessageId:      message.MessageID,
			ConversationId: resolvedID,
			SenderId:       message.SenderID,
			RecvId:         resolvedID,
			Seq:            message.Seq,
			CType:          message.Type,
			Content:        message.Content,
			VideoId:        message.VideoID,
			SendTime:       message.SendTime,
			ClientMsgId:    clientMsgID,
			Status:         message.Status,
			AttachmentId:   message.AttachmentID,
			Width:          message.Width,
			Height:         message.Height,
			DurationMs:     message.DurationMs,
			StickerId:      message.StickerID,
			PackId:         message.PackID,
			HasVideoTime:   message.VideoTime != nil,
			VideoTime:      message.VideoTime,
		})
	}

	return events
}

func (ma *MessageApplication) normalizeHistoryLimit(limit int) int {
	defaultLimit := ma.config.Message.HistoryDefaultLimit
	if defaultLimit <= 0 {
		defaultLimit = 20
	}
	maxLimit := ma.config.Message.HistoryMaxLimit
	if maxLimit <= 0 {
		maxLimit = 30
	}
	if limit <= 0 || limit > maxLimit {
		return defaultLimit
	}
	return limit
}

func (ma *MessageApplication) resolveHistoryConversationID(
	ctx context.Context,
	conversationId string,
	userId string,
) (string, error) {
	resolvedID, _, err := ma.resolveHistoryConversation(ctx, conversationId, userId)
	return resolvedID, err
}

func (ma *MessageApplication) resolveHistoryConversation(
	ctx context.Context,
	conversationId string,
	userId string,
) (string, *conversationentity.Conversation, error) {
	resolvedID := conversationId
	conv, err := ma.conversationRepository.GetByID(ctx, resolvedID)
	if err != nil {
		return "", nil, err
	}

	if conv == nil {
		normalizedID := normalizePrivateConversationID(conversationId, userId)
		if normalizedID != conversationId {
			conv, err = ma.conversationRepository.GetByID(ctx, normalizedID)
			if err != nil {
				return "", nil, err
			}
			resolvedID = normalizedID
		}
	}

	if conv == nil {
		return "", nil, ErrConversationNotFound
	}

	allowed, err := ma.canAccessConversation(ctx, conv, userId)
	if err != nil {
		return "", nil, err
	}
	if !allowed {
		return "", nil, ErrForbidden
	}

	return resolvedID, conv, nil
}

func (ma *MessageApplication) ResolveAccessibleConversationID(
	ctx context.Context,
	conversationId string,
	userId string,
) (string, error) {
	return ma.resolveHistoryConversationID(ctx, conversationId, userId)
}

func (ma *MessageApplication) ListActiveRoomIDs(userID string) ([]string, error) {
	if ma.roomUserRepository == nil {
		return nil, errors.New("房间成员仓储未配置")
	}
	return ma.roomUserRepository.ListActiveRoomIDs(userID)
}

func normalizePrivateConversationID(conversationId string, userId string) string {
	parts := strings.Split(conversationId, "_")
	if len(parts) != 2 {
		return conversationId
	}
	if parts[0] != userId && parts[1] != userId {
		return conversationId
	}

	return conversationentity.GetConversationID(
		parts[0],
		parts[1],
		int(conversationvo.PrivateChat),
	)
}

func (ma *MessageApplication) canAccessConversation(
	ctx context.Context,
	conv *conversationentity.Conversation,
	userId string,
) (bool, error) {
	switch conv.Convtype {
	case conversationvo.PrivateChat:
		return conv.UserId1 == userId || conv.UserId2 == userId, nil
	case conversationvo.RoomChat:
		if ma.roomUserRepository == nil {
			return false, ErrForbidden
		}
		err := ma.isRoomConvMember(ctx, userId, conv.RoomId)
		if err != nil {
			if errors.Is(err, roomentity.ErrMemberNotFound) || errors.Is(err, ErrNotRoomMember) {
				return false, nil
			}
			if errors.Is(err, ErrForbidden) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	default:
		return false, ErrUnknownConversationType
	}
}

func (ma *MessageApplication) getUsername(userId string) string {
	if userId == "" || ma.userRepository == nil {
		return ""
	}
	user, err := ma.userRepository.FindByUserID(userId)
	if err != nil || user == nil {
		return ""
	}
	return user.UserName
}

func userProfileFromEntity(user userentity.User) *usercach.UserProfile {
	return &usercach.UserProfile{
		Found:    true,
		UserID:   user.UserId,
		UserName: user.UserName,
		NickName: user.NickName,
		Avatar:   user.Avatar,
		Status:   int(user.Status),
	}
}

func (ma *MessageApplication) loadReadUserProfiles(
	ctx context.Context,
	userIDs []string,
) (map[string]*usercach.UserProfile, error) {
	profiles := make(map[string]*usercach.UserProfile, len(userIDs))
	missUserIDs := make([]string, 0, len(userIDs))
	missSeen := make(map[string]struct{}, len(userIDs))

	cachedProfiles := map[string]*usercach.UserProfile{}
	if ma.userCache != nil {
		cachedProfiles = ma.userCache.GetUserProfiles(ctx, userIDs)
	}
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		if profile, ok := cachedProfiles[userID]; ok {
			if profile == nil || !profile.Found {
				return nil, ErrUserNotFonund
			}
			profiles[userID] = profile
			continue
		}
		if _, exists := missSeen[userID]; exists {
			continue
		}
		missSeen[userID] = struct{}{}
		missUserIDs = append(missUserIDs, userID)
	}

	if len(missUserIDs) == 0 {
		return profiles, nil
	}
	if ma.userRepository == nil {
		return nil, fmt.Errorf("用户仓储未配置")
	}

	users, err := ma.userRepository.FindByUserIDs(missUserIDs)
	if err != nil {
		return nil, err
	}

	missSet := make(map[string]struct{}, len(missUserIDs))
	for _, userID := range missUserIDs {
		missSet[userID] = struct{}{}
	}
	loadedProfiles := make([]*usercach.UserProfile, 0, len(users))
	for _, user := range users {
		userProfile := userProfileFromEntity(user)
		profiles[userProfile.UserID] = userProfile
		loadedProfiles = append(loadedProfiles, userProfile)
		delete(missSet, userProfile.UserID)
	}

	missingUserIDs := make([]string, 0, len(missSet))
	for userID := range missSet {
		missingUserIDs = append(missingUserIDs, userID)
	}
	if ma.userCache != nil {
		cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()

		cacheTTL := time.Duration(ma.config.Cache.UserProfile.TTL) * time.Second
		if cacheTTL <= 0 {
			cacheTTL = 10 * time.Minute
		}
		if len(loadedProfiles) > 0 {
			if err := ma.userCache.SetUserProfiles(cacheCtx, loadedProfiles, cacheTTL); err != nil {
				log.Printf("批量写入用户资料缓存失败: err=%v", err)
			}
		}

		negativeTTL := time.Duration(ma.config.Cache.UserProfile.NegativeTTL) * time.Second
		if negativeTTL <= 0 {
			negativeTTL = 2 * time.Minute
		}
		if len(missingUserIDs) > 0 {
			if err := ma.userCache.SetUserProfilesNotFound(cacheCtx, missingUserIDs, negativeTTL); err != nil {
				log.Printf("批量写入用户不存在缓存失败: err=%v", err)
			}
		}
	}

	for _, userID := range userIDs {
		profile := profiles[userID]
		if profile == nil || !profile.Found {
			return nil, ErrUserNotFonund
		}
	}
	return profiles, nil
}

func (ma *MessageApplication) fillAttachmentIDs(ctx context.Context, messages []MessageDTO) error {
	if len(messages) == 0 || ma.messageAttachmentsRepository == nil {
		return nil
	}

	messageIDs := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.MessageID != "" {
			messageIDs = append(messageIDs, message.MessageID)
		}
	}
	attachments, err := ma.messageAttachmentsRepository.BatchGetByMessageIDs(ctx, messageIDs, true)
	if err != nil {
		return err
	}

	for i := range messages {
		if attachment := attachments[messages[i].MessageID]; attachment != nil {
			messages[i].AttachmentID = attachment.AttachmentId
		}
	}
	return nil
}

func (ma *MessageApplication) GetVideoDanmaku(
	ctx context.Context,
	roomId string,
	userId string,
	videoId string,
	startTime int64,
	endTime int64,
	limit int,
) ([]DanmakuDTO, error) {
	if err := ma.isRoomConvMember(ctx, userId, roomId); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if startTime < 0 {
		startTime = 0
	}

	msgs, err := ma.messageRepository.GetDanmakuByRoomVideo(ctx, roomId, videoId, startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return []DanmakuDTO{}, nil
	}

	res := make([]DanmakuDTO, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil || msg.Type != messagevo.Text || msg.VideoTime == nil {
			continue
		}

		res = append(res, DanmakuDTO{
			MessageID: msg.MessageId,
			SenderID:  msg.SenderId,
			Content:   msg.Content,
			Seq:       msg.Seq,
			TimeMs:    *msg.VideoTime,
			SendTime:  msg.SendTime,
		})
	}

	return res, nil
}
func (ma *MessageApplication) GetRoomVideoHistory(

	ctx context.Context,
	roomId string,
	userId string,
	limit int,
) ([]RoomVideoHistoryDTO, error) {
	if err := ma.isRoomConvMember(ctx, userId, roomId); err != nil {
		return nil, err
	}

	items, err := ma.messageRepository.GetRoomVideoHistory(ctx, roomId, limit)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []RoomVideoHistoryDTO{}, nil
	}

	res := make([]RoomVideoHistoryDTO, 0, len(items))
	for _, item := range items {
		if item == nil || item.VideoId == "" {
			continue
		}

		videoTime := item.VideoTime
		fileName := item.VideoId
		messageCount, err := ma.messageRepository.CountRoomVideoMessages(ctx, roomId, item.VideoId)
		if err != nil {
			return nil, err
		}
		if file, err := ma.fileRepository.FindFileByID(ctx, item.VideoId); err == nil && file != nil && file.FileName != "" {
			fileName = file.FileName
		}

		res = append(res, RoomVideoHistoryDTO{
			VideoID:        item.VideoId,
			FileName:       fileName,
			LatestSendTime: item.SendTime,
			VideoTime:      videoTime,
			MessageCount:   messageCount,
		})
	}

	return res, nil
}
