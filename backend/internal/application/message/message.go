package message

import (
	"IM_backend/configs"
	idport "IM_backend/internal/application/ports/id"
	outboxport "IM_backend/internal/application/ports/outbox"
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
	messageRepository            messagerepo.MessageRepository
	txManager                    txmanager.TxManager
	userCache                    usercach.UserCache
	userConversationRepository   conversationrepo.UserConversationRepository
	conversationRepository       conversationrepo.ConversationRepository
	friendRepository             friendrepo.FriendRepository
	fileRepository               filerepo.FileRepository
	objectStorage                objectstorage.ObjectStorage
	messageImageRepository       messagerepo.MessageImageRepository
	messageFileRepository        messagerepo.MessageFileRepository
	messageStickerRepository     messagerepo.MessageStickerRepository
	messageVideoRepository       messagerepo.MessageVideoRepository
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
	userCache usercach.UserCache,
	txManager txmanager.TxManager,
	userConversationRepository conversationrepo.UserConversationRepository,
	conversationRepository conversationrepo.ConversationRepository,
	messageOutboxRepository outboxport.Repository,
	friendRepository friendrepo.FriendRepository,
	fileRepository filerepo.FileRepository,
	objectStorage objectstorage.ObjectStorage,
	messageImageRepository messagerepo.MessageImageRepository,
	messageFileRepository messagerepo.MessageFileRepository,
	messageStickerRepository messagerepo.MessageStickerRepository,
	messageVideoRepository messagerepo.MessageVideoRepository,
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
		objectStorage:              objectStorage,
		messageImageRepository:     messageImageRepository,
		messageFileRepository:      messageFileRepository,
		messageStickerRepository:   messageStickerRepository,
		messageVideoRepository:     messageVideoRepository,
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
	conversationId string,
	lastReadSeq int64,
) error {
	uconv, err := ma.userConversationRepository.GetUserConversation(ctx, userId, conversationId)
	if err != nil {
		return ErrConversationNotFound
	}

	oldLastReadSeq := uconv.LastReadSeq
	if lastReadSeq <= oldLastReadSeq {
		return nil
	}

	conv, err := ma.conversationRepository.GetByID(ctx, conversationId)
	if err != nil {
		return err
	}
	if conv == nil {
		return ErrConversationNotFound
	}
	if lastReadSeq > conv.LatestSeq {
		lastReadSeq = conv.LatestSeq
	}
	if lastReadSeq <= oldLastReadSeq {
		return nil
	}

	notifyUserIds, err := ma.messageRepository.ListDistinctSendersBySeqRange(
		ctx,
		conversationId,
		oldLastReadSeq,
		lastReadSeq,
		userId,
	)
	if err != nil {
		return err
	}

	uconv.UpdateReadSeq(lastReadSeq)

	if ma.txManager == nil || ma.messageOutboxRepository == nil {
		return fmt.Errorf("消息出箱组件未配置")
	}

	sfKey := "user-profile:" + userId
	result := ma.sf.DoChan(sfKey, func() (any, error) {
		ctx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			3*time.Second,
		)
		defer cancel()

		var (
			user        *userentity.User
			userProfile *usercach.UserProfile
			exist       bool
			err         error
		)

		userProfile, exist, err = ma.userCache.GetUserProfile(ctx, userId)

		if err != nil {
			log.Println("获取用户资料缓存失败：", err)
		}

		if !exist {
			// 缓存中不存在，回查数据库
			user, err = ma.userRepository.FindByUserID(userId)

			if err != nil {
				return nil, err
			}

			if user == nil {
				// 防止缓存穿透
				ma.userCache.SetUserProfileNotFound(
					ctx,
					userId,
					time.Duration(ma.config.Cache.UserProfile.NegativeTTL)*time.Second,
				)
				return nil, ErrUserNotFonund
			}

			userProfile = userProfileFromEntity(*user)

			if err := ma.userCache.SetUserProfile(
				ctx,
				userProfile,
				time.Duration(ma.config.Cache.UserProfile.TTL)*time.Second,
			); err != nil {
				log.Println("写入用户资料缓存失败：", err)
			}

			return userProfile, nil
		}

		if !userProfile.Found {
			return nil, ErrUserNotFonund
		}

		return userProfile, nil
	})

	var userProfile *usercach.UserProfile
	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-result:
		if result.Err != nil {
			return result.Err
		}

		state, ok := result.Val.(*usercach.UserProfile)
		if !ok || state == nil {
			// 缓存不生效，需要删除当前缓存
			ma.userCache.DeleteUserProfiles(ctx, []string{userId})
			return ErrUserNotFonund
		}

		userProfile = state
	}

	return ma.txManager.WithinTransaction(ctx, func(tx any) error {
		advanced, err := ma.userConversationRepository.WithTx(tx).AdvanceReadSeq(ctx, uconv)
		if err != nil {
			return err
		}
		if !advanced {
			return nil
		}

		if len(notifyUserIds) == 0 {
			return nil
		}

		readEvent := protocol.MessageReadCommittedEvent{
			ReaderId:       userId,
			ConversationId: conversationId,
			OldReadSeq:     oldLastReadSeq,
			LastReadSeq:    lastReadSeq,
			ConvType:       protocol.ConvType(conv.Convtype),
			NotifyUserIds:  notifyUserIds,
			Avatar:         userProfile.Avatar,
		}

		eventPayload, err := json.Marshal(readEvent)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(protocol.Envelope{
			To:      conversationId,
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

func (ma *MessageApplication) normalizeMediaDTO(ctx context.Context, dto *MessageAppeDTO) error {
	if dto == nil {
		return nil
	}
	switch messagevo.CType(dto.CType) {
	case messagevo.Image, messagevo.Video, messagevo.File:
		if dto.FileId == "" || ma.fileRepository == nil {
			return fmt.Errorf("文件引用不能为空")
		}
		file, err := ma.fileRepository.FindUploadedByIDForUploader(ctx, dto.FileId, dto.SendId)
		if err != nil {
			return err
		}
		if file == nil {
			return fmt.Errorf("文件不存在或不可用：%s", dto.FileId)
		}

		// 不再相信前端传递过来的媒体文件元信息
		dto.FileName = file.FileName
		dto.FileSize = file.Size
		dto.MimeType = file.ContentType
		dto.Content = file.FileName
	case messagevo.Sticker:
		if dto.StickerId == "" {
			return fmt.Errorf("表情内容不能为空")
		}
	}
	return nil
}

func (ma *MessageApplication) buildAttachment(dto *MessageAppeDTO, messageId string) (*messageentity.MessageAttachment, error) {
	if ma.messageAttachmentsRepository == nil {
		return nil, fmt.Errorf("消息附件仓储未配置")
	}
	attachmentId, err := ma.idGenerator.Generate()
	if err != nil {
		return nil, err
	}
	dto.AttachmentId = attachmentId
	attachmentTTL := ma.config.Message.AttachmentTTLSeconds
	if attachmentTTL <= 0 {
		return nil, errors.New("消息附件有效期配置无效")
	}
	now := time.Now().UnixMilli()
	return messageentity.NewMessageAttachment(
		attachmentId,
		messageId,
		dto.ConversationID,
		dto.FileId,
		int8(dto.CType),
		now+attachmentTTL*int64(time.Second/time.Millisecond),
		now,
	), nil
}

func (ma *MessageApplication) buildMediaWriter(dto *MessageAppeDTO, messageId string) (func(context.Context, any) error, error) {
	if dto == nil {
		return nil, nil
	}

	switch messagevo.CType(dto.CType) {
	case messagevo.Image:
		if ma.messageImageRepository == nil {
			return nil, fmt.Errorf("消息图片仓储未配置")
		}
		if dto.FileId == "" {
			return nil, fmt.Errorf("图片内容不能为空")
		}
		attachment, err := ma.buildAttachment(dto, messageId)
		if err != nil {
			return nil, err
		}
		item := messageentity.NewMessageImage(
			messageId,
			dto.MimeType,
			dto.Width,
			dto.Height,
		)
		return func(ctx context.Context, tx any) error {
			if err := ma.messageImageRepository.WithTx(tx).Create(ctx, item); err != nil {
				return err
			}
			return ma.messageAttachmentsRepository.WithTx(tx).Create(ctx, attachment)
		}, nil
	case messagevo.File:
		if ma.messageFileRepository == nil {
			return nil, fmt.Errorf("消息文件仓储未配置")
		}
		if dto.FileId == "" {
			return nil, fmt.Errorf("文件内容不能为空")
		}
		attachment, err := ma.buildAttachment(dto, messageId)
		if err != nil {
			return nil, err
		}
		item := messageentity.NewMessageFile(
			messageId,
			dto.FileName,
		)
		return func(ctx context.Context, tx any) error {
			if err := ma.messageFileRepository.WithTx(tx).Create(ctx, item); err != nil {
				return err
			}
			return ma.messageAttachmentsRepository.WithTx(tx).Create(ctx, attachment)
		}, nil
	case messagevo.Sticker:
		if ma.messageStickerRepository == nil {
			return nil, fmt.Errorf("消息表情仓储未配置")
		}
		if dto.StickerId == "" {
			return nil, fmt.Errorf("表情内容不能为空")
		}
		item := messageentity.NewMessageSticker(
			messageId,
			dto.StickerId,
			dto.PackId,
			dto.Width,
			dto.Height,
		)
		return func(ctx context.Context, tx any) error {
			return ma.messageStickerRepository.WithTx(tx).Create(ctx, item)
		}, nil
	case messagevo.Video:
		if ma.messageVideoRepository == nil {
			return nil, fmt.Errorf("消息视频仓储未配置")
		}
		if dto.FileId == "" {
			return nil, fmt.Errorf("视频内容不能为空")
		}
		attachment, err := ma.buildAttachment(dto, messageId)
		if err != nil {
			return nil, err
		}
		duration := int64(0)
		if dto.DurationMs != nil {
			duration = *dto.DurationMs
		}
		item := messageentity.NewMessageVideo(
			messageId,
			duration,
			dto.Width,
			dto.Height,
		)
		return func(ctx context.Context, tx any) error {
			if err := ma.messageVideoRepository.WithTx(tx).Create(ctx, item); err != nil {
				return err
			}
			return ma.messageAttachmentsRepository.WithTx(tx).Create(ctx, attachment)
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

func (ma *MessageApplication) checkConvMember(ctx context.Context, dto MessageAppeDTO) error {
	switch conversationvo.ConvType(dto.ConvType) {
	case conversationvo.PrivateChat:
		return ma.isPrivateConvMember(ctx, dto.SendId, dto.RecvId)
	case conversationvo.RoomChat:
		return ma.isRoomConvMember(ctx, dto.SendId, dto.RecvId)
	default:
		return ErrConversationNotFound
	}
}

func (ma *MessageApplication) validateFileReference(dto MessageAppeDTO) error {
	if dto.FileId == "" {
		return errors.New("文件 ID 不能为空")
	}

	if len(dto.FileId) > ma.config.Message.MaxIdentifierLength {
		return errors.New("文件 ID 无效")
	}

	return nil
}

// 根据消息类型检验消息内容
func (ma *MessageApplication) validateMessageCType(dto MessageAppeDTO) error {
	if dto.SendId == "" || dto.RecvId == "" {
		return errors.New("发送方或接收方不能为空")
	}

	switch messagevo.CType(dto.CType) {
	case messagevo.Text:
		content := strings.TrimSpace(dto.Content)
		if content == "" {
			return errors.New("文本内容不能为空")
		}

		if utf8.RuneCountInString(content) > ma.config.Message.MaxTextRunes {
			return errors.New("文本内容过长")
		}

		// 文本消息，其他的媒体消息字段都为空
		if dto.FileId != "" ||
			dto.StickerId != "" ||
			dto.PackId != "" ||
			dto.Width != 0 ||
			dto.Height != 0 ||
			dto.DurationMs != nil {
			return errors.New("文本消息包含非法媒体字段")
		}
	case messagevo.Image:
		if err := ma.validateFileReference(dto); err != nil {
			return err
		}

		if dto.Width <= 0 || dto.Width > ma.config.Message.MaxWidth ||
			dto.Height <= 0 || dto.Height > ma.config.Message.MaxHeight {
			return errors.New("图片尺寸无效")
		}

		if dto.DurationMs != nil ||
			dto.StickerId != "" ||
			dto.PackId != "" {
		}
	case messagevo.File:
		ma.validateFileReference(dto)

		if dto.Width != 0 ||
			dto.Height != 0 ||
			dto.DurationMs != nil ||
			dto.StickerId != "" ||
			dto.PackId != "" {
			return errors.New("文件消息包含非法媒体字段")
		}

	case messagevo.Sticker:
		if dto.StickerId == "" {
			return errors.New("表情 ID 不能为空")
		}
		if len(dto.StickerId) > ma.config.Message.MaxIdentifierLength || len(dto.PackId) > ma.config.Message.MaxIdentifierLength {
			return errors.New("表情标识过长")
		}
	}

	return nil
}

func (ma *MessageApplication) HandleSendMessage(ctx context.Context, dto MessageAppeDTO) (*MessageAppeDTO, error) {
	conversationId := conversationentity.GetConversationID(dto.SendId, dto.RecvId, dto.ConvType)
	dto.ConversationID = conversationId

	if err := ma.validateMessageCType(dto); err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	requestHash, err := buildMessageRequestHash(dto)
	if err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	// 幂等检查：同一 clientMsgId 在 5 分钟内只处理一次
	if dto.ClientMsgId != "" && ma.messageCache != nil {
		if entry, err := ma.messageCache.GetDedupEntry(ctx, dto.SendId, dto.ClientMsgId); err == nil && entry.MessageID != "" {
			if entry.RequestHash != requestHash {
				return &MessageAppeDTO{ClientMsgId: dto.ClientMsgId, Status: string(protocol.AckStatusFailed)}, messageentity.ErrClientMessageConflict
			}
			return &MessageAppeDTO{
				ClientMsgId:    dto.ClientMsgId,
				ConversationID: conversationId,
				MessageId:      entry.MessageID,
				Status:         string(protocol.AckStatusSent),
			}, nil
		}
	}

	if err := ma.checkConvMember(ctx, dto); err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	if err := ma.normalizeMediaDTO(ctx, &dto); err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	messageId, err := ma.idGenerator.Generate()
	if err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	messageType := messagevo.CType(dto.CType)
	message := messageentity.NewMessage(
		messageId,
		conversationId,
		dto.SendId,
		0,
		messageType,
		dto.Content,
		dto.VideoId,
		dto.VideoTime,
	)
	if dto.ClientMsgId != "" {
		clientMsgID := dto.ClientMsgId
		message.ClientMsgId = &clientMsgID
	}
	message.RequestHash = requestHash

	mediaWriter, err := ma.buildMediaWriter(&dto, messageId)
	if err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			MessageId:   messageId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	conv := conversationentity.NewConversation(
		conversationId,
		dto.SendId,
		dto.RecvId,
		dto.ConvType,
		0,
		messageId,
	)

	userConv := conversationentity.BuildUserConversation(
		dto.SendId,
		conversationId,
		0,
		conversationvo.ConvType(dto.ConvType),
	)

	// 弹幕需要同时满足前端发送有视频时间以及在房间内
	isDanmaku := dto.ConvType == int(conversationvo.RoomChat) && dto.VideoTime != nil
	if ma.txManager == nil || ma.messageOutboxRepository == nil {
		return nil, fmt.Errorf("消息出箱组件未配置")
	}

	value := ctx.Value("op")
	op, ok := value.(string)
	if !ok || op == "" {
		return nil, ErrUnknown
	}

	senderUsername := ma.getUsername(dto.SendId)
	messageEvent := protocol.MessageEvent{
		MessageId:      messageId,
		ConversationId: conversationId,
		SendId:         dto.SendId,
		SenderUsername: senderUsername,
		RecvId:         dto.RecvId,
		ConvType:       protocol.ConvType(dto.ConvType),
		CType:          dto.CType,
		Content:        dto.Content,
		SendTime:       message.SendTime,
		ClientMsgId:    dto.ClientMsgId,
		AttachmentId:   dto.AttachmentId,
		Width:          dto.Width,
		Height:         dto.Height,
		DurationMs:     dto.DurationMs,
		StickerId:      dto.StickerId,
		PackId:         dto.PackId,
		HasVideoTime:   dto.VideoTime != nil,
		VideoTime:      dto.VideoTime,
	}

	var nextSeq int64 = 0
	err = ma.txManager.WithinTransaction(ctx, func(tx any) error {
		msgRepo := ma.messageRepository.WithTx(tx)
		convRepo := ma.conversationRepository.WithTx(tx)
		userConvRepo := ma.userConversationRepository.WithTx(tx)
		outboxRepo := ma.messageOutboxRepository.WithTx(tx)

		if messagevo.CType(dto.CType) == messagevo.Image || messagevo.CType(dto.CType) == messagevo.Video || messagevo.CType(dto.CType) == messagevo.File {
			lockedFile, lockErr := ma.fileRepository.WithTx(tx).FindUploadedByIDForUploaderForUpdate(ctx, dto.FileId, dto.SendId)
			if lockErr != nil {
				return lockErr
			}
			if lockedFile == nil {
				return fmt.Errorf("file is missing or being deleted: %s", dto.FileId)
			}
			dto.FileName = lockedFile.FileName
			dto.FileSize = lockedFile.Size
			dto.MimeType = lockedFile.ContentType
			dto.Content = lockedFile.FileName
			message.Content = dto.Content
			messageEvent.Content = dto.Content
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
			From:    messageEvent.SendId,
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

		return nil
	})

	// 判断是否出发唯一键索引冲突
	if errors.Is(err, messageentity.ErrDuplicateClientMessage) && dto.ClientMsgId != "" {
		// 消息重复写入，尝试从数据库中找到这条消息并直接返回
		existing, findErr := ma.messageRepository.FindByClientMsgID(ctx, dto.SendId, dto.ClientMsgId)
		if findErr == nil && existing != nil {
			if !sameClientMessage(dto, existing, requestHash) {
				return &MessageAppeDTO{
					ClientMsgId: dto.ClientMsgId,
					Status:      string(protocol.AckStatusFailed),
				}, messageentity.ErrClientMessageConflict
			}
			return &MessageAppeDTO{
				ClientMsgId:    dto.ClientMsgId,
				ConversationID: existing.ConversationId,
				MessageId:      existing.MessageId,
				Seq:            existing.Seq,
				Status:         string(protocol.AckStatusSent),
			}, nil
		}
		if findErr != nil {
			return nil, findErr
		}
	}

	if err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			MessageId:   messageId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	// 标记已处理，5 分钟内同一 clientMsgId 幂等返回
	if dto.ClientMsgId != "" && ma.messageCache != nil {
		_, _ = ma.messageCache.SetDedupEntry(ctx, dto.SendId, dto.ClientMsgId, messageId, requestHash, 5*time.Minute)
	}

	return &MessageAppeDTO{
		ClientMsgId:    dto.ClientMsgId,
		ConversationID: conversationId,
		MessageId:      messageId,
		AttachmentId:   dto.AttachmentId,
		SenderUsername: senderUsername,
		Seq:            nextSeq,
		Status:         string(protocol.AckStatusSent),
	}, nil
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

func buildMessageRequestHash(dto MessageAppeDTO) (string, error) {
	content := strings.TrimSpace(dto.Content)
	switch messagevo.CType(dto.CType) {
	case messagevo.Image, messagevo.Video, messagevo.File:
		// 媒体消息的身份由 FileId 表示，不依赖文件名等数据库元数据。
		content = ""
	}

	payload, err := json.Marshal(messageRequestFingerprint{
		ConversationID: dto.ConversationID,
		CType:          dto.CType,
		Content:        content,
		FileID:         dto.FileId,
		Width:          dto.Width,
		Height:         dto.Height,
		DurationMs:     dto.DurationMs,
		StickerID:      dto.StickerId,
		PackID:         dto.PackId,
		VideoID:        dto.VideoId,
		VideoTime:      dto.VideoTime,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func sameClientMessage(dto MessageAppeDTO, existing *messageentity.Message, requestHash string) bool {
	if existing == nil {
		return false
	}
	if existing.RequestHash != "" {
		return existing.RequestHash == requestHash
	}
	return existing.ConversationId == conversationentity.GetConversationID(dto.SendId, dto.RecvId, dto.ConvType) &&
		existing.Type == messagevo.CType(dto.CType) &&
		existing.Content == dto.Content &&
		existing.VideoId == dto.VideoId &&
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
) ([]MessageAppeDTO, int64, bool, error) {
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
		return []MessageAppeDTO{}, -1, false, nil
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

	msgsApp := toMessagesAppDTO(msgs)
	ma.fillSenderUsernames(ctx, msgsApp)
	if err := ma.fillMediaFields(ctx, msgsApp); err != nil {
		return nil, -1, false, err
	}

	sort.Slice(msgsApp, func(i, j int) bool {
		return msgsApp[i].Seq < msgsApp[j].Seq
	})

	return msgsApp, nextCursor, hasMore, nil
}

func (ma *MessageApplication) SyncMessages(
	ctx context.Context,
	conversationId string,
	userId string,
	afterSeq int64,
	limit int,
) ([]MessageAppeDTO, int64, bool, error) {
	limit = ma.normalizeSyncLimit(limit)
	if afterSeq < 0 {
		afterSeq = 0
	}

	resolvedConversationID, err := ma.resolveHistoryConversationID(ctx, conversationId, userId)
	if err != nil {
		return nil, afterSeq, false, err
	}

	msgs, err := ma.messageRepository.ListAfterSeq(
		ctx,
		resolvedConversationID,
		afterSeq,
		limit+1,
	)
	if err != nil {
		return nil, afterSeq, false, err
	}

	sort.SliceStable(msgs, func(i, j int) bool {
		if msgs[i] == nil {
			return false
		}
		if msgs[j] == nil {
			return true
		}
		return msgs[i].Seq < msgs[j].Seq
	})

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	msgsApp := toMessagesAppDTO(msgs)
	sort.SliceStable(msgsApp, func(i, j int) bool {
		return msgsApp[i].Seq < msgsApp[j].Seq
	})
	ma.fillSenderUsernames(ctx, msgsApp)
	if err := ma.fillMediaFields(ctx, msgsApp); err != nil {
		return nil, afterSeq, false, err
	}

	nextSeq := afterSeq
	if len(msgsApp) > 0 {
		nextSeq = msgsApp[len(msgsApp)-1].Seq
	}

	return msgsApp, nextSeq, hasMore, nil
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

func (ma *MessageApplication) normalizeSyncLimit(limit int) int {
	defaultLimit := ma.config.Message.SyncDefaultLimit
	if defaultLimit <= 0 {
		defaultLimit = 50
	}
	maxLimit := ma.config.Message.SyncMaxLimit
	if maxLimit <= 0 {
		maxLimit = 100
	}
	if defaultLimit > maxLimit {
		defaultLimit = maxLimit
	}
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func (ma *MessageApplication) resolveHistoryConversationID(
	ctx context.Context,
	conversationId string,
	userId string,
) (string, error) {
	resolvedID := conversationId
	conv, err := ma.conversationRepository.GetByID(ctx, resolvedID)
	if err != nil {
		return "", err
	}

	if conv == nil {
		normalizedID := normalizePrivateConversationID(conversationId, userId)
		if normalizedID != conversationId {
			conv, err = ma.conversationRepository.GetByID(ctx, normalizedID)
			if err != nil {
				return "", err
			}
			resolvedID = normalizedID
		}
	}

	if conv == nil {
		return "", ErrConversationNotFound
	}

	allowed, err := ma.canAccessConversation(ctx, conv, userId)
	if err != nil {
		return "", err
	}
	if !allowed {
		return "", ErrForbidden
	}

	return resolvedID, nil
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

// 批量回填 userName，用于前端展示
func (ma *MessageApplication) fillSenderUsernames(ctx context.Context, messages []MessageAppeDTO) {
	if len(messages) == 0 || ma.userRepository == nil {
		return
	}
	userIds := uniqueSenderIDs(messages)
	profiles := ma.loadUserProfiles(ctx, userIds)
	for i := range messages {
		if profile := profiles[messages[i].SendId]; profile != nil {
			messages[i].SenderUsername = profile.UserName
		}
	}
}

func uniqueSenderIDs(messages []MessageAppeDTO) []string {
	seen := make(map[string]struct{}, len(messages))
	userIDs := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.SendId == "" {
			continue
		}
		if _, ok := seen[msg.SendId]; ok {
			continue
		}
		seen[msg.SendId] = struct{}{}
		userIDs = append(userIDs, msg.SendId)
	}
	return userIDs
}

func (ma *MessageApplication) loadUserProfiles(ctx context.Context, userIDs []string) map[string]*usercach.UserProfile {
	profiles := make(map[string]*usercach.UserProfile, len(userIDs))
	if len(userIDs) == 0 {
		return profiles
	}

	missUserIds := userIDs
	if ma.userCache != nil {
		cachedProfiles := ma.userCache.GetUserProfiles(ctx, userIDs)
		missUserIds = make([]string, 0, len(userIDs))
		for _, userId := range userIDs {
			profile := cachedProfiles[userId]
			if profile == nil {
				missUserIds = append(missUserIds, userId)
				continue
			}
			if !profile.Found {
				continue
			}
			profiles[userId] = profile
		}
	}

	if len(missUserIds) == 0 {
		return profiles
	}
	missSet := make(map[string]struct{}, len(missUserIds))
	for _, userId := range missUserIds {
		missSet[userId] = struct{}{}
	}

	users, err := ma.userRepository.FindByUserIDs(missUserIds)
	if err != nil {
		return profiles
	}

	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	cacheTTL := time.Duration(ma.config.Cache.UserProfile.TTL) * time.Second
	if cacheTTL <= 0 {
		cacheTTL = 10 * time.Minute
	}

	for _, user := range users {
		// 设置能够成功查询到的用户的信息缓存
		profile := userProfileFromEntity(user)
		profiles[user.UserId] = profile
		delete(missSet, user.UserId)
		if ma.userCache != nil {
			_ = ma.userCache.SetUserProfile(cacheCtx, profile, cacheTTL)
		}
	}

	negativeTTL := time.Duration(ma.config.Cache.UserProfile.NegativeTTL) * time.Second
	if negativeTTL <= 0 {
		negativeTTL = 2 * time.Minute
	}
	if ma.userCache != nil {
		for userId := range missSet {
			_ = ma.userCache.SetUserProfileNotFound(cacheCtx, userId, negativeTTL)
		}
	}

	return profiles
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

func (ma *MessageApplication) fillMediaFields(ctx context.Context, messages []MessageAppeDTO) error {
	if len(messages) == 0 {
		return nil
	}

	// Collect message IDs grouped by type
	imageIDs := make([]string, 0)
	videoIDs := make([]string, 0)
	messageFileIDs := make([]string, 0)
	stickerIDs := make([]string, 0)

	for _, m := range messages {
		switch messagevo.CType(m.CType) {
		case messagevo.Image:
			imageIDs = append(imageIDs, m.MessageId)
		case messagevo.Video:
			videoIDs = append(videoIDs, m.MessageId)
		case messagevo.File:
			messageFileIDs = append(messageFileIDs, m.MessageId)
		case messagevo.Sticker:
			stickerIDs = append(stickerIDs, m.MessageId)
		}
	}

	// Batch-fetch from sub-repositories
	var (
		images      map[string]*messageentity.MessageImage
		videos      map[string]*messageentity.MessageVideo
		files       map[string]*messageentity.MessageFile
		stickers    map[string]*messageentity.MessageSticker
		attachments map[string]*messageentity.MessageAttachment
	)
	var err error

	if len(imageIDs) > 0 && ma.messageImageRepository != nil {
		images, err = ma.messageImageRepository.BatchGetByMessageIDs(ctx, imageIDs)
		if err != nil {
			return err
		}
	}
	if len(videoIDs) > 0 && ma.messageVideoRepository != nil {
		videos, err = ma.messageVideoRepository.BatchGetByMessageIDs(ctx, videoIDs)
		if err != nil {
			return err
		}
	}
	if len(messageFileIDs) > 0 && ma.messageFileRepository != nil {
		files, err = ma.messageFileRepository.BatchGetByMessageIDs(ctx, messageFileIDs)
		if err != nil {
			return err
		}
	}
	if len(stickerIDs) > 0 && ma.messageStickerRepository != nil {
		stickers, err = ma.messageStickerRepository.BatchGetByMessageIDs(ctx, stickerIDs)
		if err != nil {
			return err
		}
	}
	if ma.messageAttachmentsRepository != nil {
		messageIDs := make([]string, 0, len(messages))
		for _, message := range messages {
			messageIDs = append(messageIDs, message.MessageId)
		}
		attachments, err = ma.messageAttachmentsRepository.BatchGetByMessageIDs(ctx, messageIDs)
		if err != nil {
			return err
		}
	}

	attachmentFileIDs := make([]string, 0, len(attachments))
	seenFileIDs := make(map[string]struct{}, len(attachments))
	for _, attachment := range attachments {
		if attachment == nil || attachment.FileId == "" {
			continue
		}
		if _, exists := seenFileIDs[attachment.FileId]; exists {
			continue
		}
		seenFileIDs[attachment.FileId] = struct{}{}
		attachmentFileIDs = append(attachmentFileIDs, attachment.FileId)
	}
	fileObjects := make(map[string]*fileentity.File, len(attachmentFileIDs))
	if len(attachmentFileIDs) > 0 {
		if ma.fileRepository == nil {
			return fmt.Errorf("文件仓储未配置")
		}
		fileObjects, err = ma.fileRepository.BatchGetByIDs(ctx, attachmentFileIDs)
		if err != nil {
			return err
		}
	}

	for i := range messages {
		if attachment, ok := attachments[messages[i].MessageId]; ok && attachment != nil {
			messages[i].AttachmentId = attachment.AttachmentId
			messages[i].FileId = attachment.FileId
			if file := fileObjects[attachment.FileId]; file != nil {
				messages[i].FileName = file.FileName
				messages[i].FileSize = file.Size
				messages[i].MimeType = file.ContentType
			}
		}
		switch messagevo.CType(messages[i].CType) {
		case messagevo.Image:
			if img, ok := images[messages[i].MessageId]; ok && img != nil {
				messages[i].Width = img.Width
				messages[i].Height = img.Height
			}
		case messagevo.Video:
			if vid, ok := videos[messages[i].MessageId]; ok && vid != nil {
				messages[i].Width = vid.Width
				messages[i].Height = vid.Height
				messages[i].DurationMs = &vid.DurationMs
			}
		case messagevo.File:
			if f, ok := files[messages[i].MessageId]; ok && f != nil {
				if f.DownloadName != "" {
					messages[i].FileName = f.DownloadName
				}
			}
		case messagevo.Sticker:
			if s, ok := stickers[messages[i].MessageId]; ok && s != nil {
				messages[i].StickerId = s.StickerId
				messages[i].PackId = s.PackId
				messages[i].Width = s.Width
				messages[i].Height = s.Height
			}
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
			MessageId: msg.MessageId,
			SenderId:  msg.SendId,
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
		if file, err := ma.fileRepository.GetByID(ctx, item.VideoId); err == nil && file != nil && file.FileName != "" {
			fileName = file.FileName
		}

		res = append(res, RoomVideoHistoryDTO{
			VideoId:        item.VideoId,
			FileName:       fileName,
			LatestSendTime: item.SendTime,
			VideoTime:      videoTime,
			MessageCount:   messageCount,
		})
	}

	return res, nil
}
