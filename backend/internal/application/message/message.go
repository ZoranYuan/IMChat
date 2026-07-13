package message

import (
	"IM_backend/configs"
	mqport "IM_backend/internal/application/ports/mq"
	convcache "IM_backend/internal/application/ports/persistence/cache/conversation"
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	roomvo "IM_backend/internal/domain/room/value_object"
	userentity "IM_backend/internal/domain/user/entity"
	"IM_backend/internal/infrastructure/id/snow"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"slices"
	"sort"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

type MessageApplication struct {
	config                     configs.Config
	conversationCache          convcache.ConversationCache
	messageRepository          messagerepo.MessageRepository
	txManager                  txmanager.TxManager
	userConversationRepository messagerepo.UserConversationRepository
	conversationRepository     messagerepo.ConversationRepository
	friendRepository           friendrepo.FriendRepository
	fileRepository             filerepo.FileRepository
	messageImageRepository     messagerepo.MessageImageRepository
	messageFileRepository      messagerepo.MessageFileRepository
	messageStickerRepository   messagerepo.MessageStickerRepository
	messageVideoRepository     messagerepo.MessageVideoRepository
	messageOutboxRepository    messagerepo.MessageOutboxRepository
	userRepository             userrepo.UserRepository
	roomUserRepository         roomrepo.RoomUserRepository
	roomRepository             roomrepo.RoomRepository
	taskManager                mqport.TaskManager
	sf                         singleflight.Group
}

func NewMessageApplication(
	config configs.Config,
	conversationCache convcache.ConversationCache,
	txManager txmanager.TxManager,
	taskManager mqport.TaskManager,
	userConversationRepository messagerepo.UserConversationRepository,
	conversationRepository messagerepo.ConversationRepository,
	messageOutboxRepository messagerepo.MessageOutboxRepository,
	friendRepository friendrepo.FriendRepository,
	fileRepository filerepo.FileRepository,
	messageImageRepository messagerepo.MessageImageRepository,
	messageFileRepository messagerepo.MessageFileRepository,
	messageStickerRepository messagerepo.MessageStickerRepository,
	messageVideoRepository messagerepo.MessageVideoRepository,
	userRepository userrepo.UserRepository,
	messageRepository messagerepo.MessageRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	roomRepository roomrepo.RoomRepository,
) *MessageApplication {
	return &MessageApplication{
		config:                     config,
		conversationCache:          conversationCache,
		txManager:                  txManager,
		taskManager:                taskManager,
		messageOutboxRepository:    messageOutboxRepository,
		messageRepository:          messageRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
		userRepository:             userRepository,
		roomRepository:             roomRepository,
		roomUserRepository:         roomUserRepository,
		friendRepository:           friendRepository,
		fileRepository:             fileRepository,
		messageImageRepository:     messageImageRepository,
		messageFileRepository:      messageFileRepository,
		messageStickerRepository:   messageStickerRepository,
		messageVideoRepository:     messageVideoRepository,
	}
}

func (ma *MessageApplication) checkConvMember(
	ctx context.Context,
	convType messagevo.ConvType,
	conversationId string,
	userId string,
	recvId string,
) (bool, error) {
	isMember, cacheVersion, err := ma.conversationCache.IsMemberWithVersion(
		ctx, conversationId, userId,
	)
	if err == nil && cacheVersion > 0 && convType == messagevo.PrivateChat {
		return isMember, nil
	}

	key := userId + ":" + conversationId

	v, err, _ := ma.sf.Do(key, func() (any, error) {
		isMember, cacheVersion, err := ma.conversationCache.IsMemberWithVersion(
			ctx, conversationId, userId,
		)

		// double check
		if err == nil && cacheVersion > 0 && convType == messagevo.PrivateChat {
			return isMember, nil
		}

		if convType == messagevo.PrivateChat {
			friend, err := ma.friendRepository.FindRelation(userId, recvId)
			if err != nil {
				return false, err
			}
			if friend == nil {
				return false, ErrNotFriends
			}
			cacheErr := ma.conversationCache.SetMembers(
				ctx,
				conversationId,
				[]string{userId, recvId},
				1,
			)
			return true, cacheErr
		}

		room, err := ma.roomRepository.FindActiveRoom(recvId, int(roomvo.Activate))
		if err != nil {
			return false, err
		}

		if cacheVersion > 0 && cacheVersion == room.Version {
			return isMember, nil
		}

		members, err := ma.roomUserRepository.ListActiveUserIDs(recvId)
		if err != nil {
			return false, err
		}
		if len(members) == 0 {
			return false, ErrNotRoomMember
		}
		cacheErr := ma.conversationCache.SetMembers(
			ctx,
			conversationId,
			members,
			room.Version,
		)
		if cacheErr != nil {
			return false, cacheErr
		}
		if slices.Contains(members, userId) {
			return true, nil
		}

		return false, ErrNotRoomMember
	})

	if err != nil {
		return false, err
	}

	res, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("type assert failed")
	}

	return res, nil
}

func (ma *MessageApplication) HandleMessageReadAck(
	ctx context.Context,
	userId string,
	conversationId string,
	lastReadSeq int64,
	senderId string,
) error {
	if senderId == "" {
		return nil
	}

	uconv, err := ma.userConversationRepository.GetUserConversation(ctx, userId, conversationId)
	if err != nil {
		return ErrConversationNotFound
	}

	if lastReadSeq <= uconv.LastReadSeq {
		return nil
	}

	// 查会话类型，前端需要此字段来区分展示
	conv, err := ma.conversationRepository.GetByID(ctx, conversationId)
	if err != nil {
		return err
	}
	if conv == nil {
		return ErrConversationNotFound
	}
	uconv.UpdateReadSeq(lastReadSeq)

	if ma.txManager == nil || ma.messageOutboxRepository == nil {
		return fmt.Errorf("message outbox is not configured")
	}
	return ma.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		if err := ma.userConversationRepository.WithTx(tx).UpdateReadSeq(ctx, uconv); err != nil {
			return err
		}
		if senderId == userId {
			return nil
		}
		avatar := ""
		if conv.Convtype == messagevo.RoomChat && ma.userRepository != nil {
			if user, err := ma.userRepository.FindByUserID(userId); err == nil && user != nil {
				avatar = user.Avatar
			}
		}
		ackEvent := protocol.MessageReadAckEvent{
			ConversationId: conversationId,
			LastReadSeq:    lastReadSeq,
			UserId:         userId,
			ConvType:       protocol.ConvType(conv.Convtype),
			SenderId:       senderId,
			Avatar:         avatar,
		}
		payload, err := json.Marshal(ackEvent)
		if err != nil {
			return err
		}
		outbox := &messageentity.MessageOutbox{EventType: string(protocol.EventMessageReadAck), Topic: string(protocol.EventMessageReadAck), MessageKey: userId + ":" + conversationId, Payload: payload}
		return ma.messageOutboxRepository.WithTx(tx).Create(ctx, outbox)
	})
}

func (ma *MessageApplication) publishMessageReadAck(
	ctx context.Context,
	userId string,
	conversationId string,
	lastReadSeq int64,
	senderId string,
	convType int,
) {
	avatar := ""
	if ma.userRepository != nil {
		if user, err := ma.userRepository.FindByUserID(userId); err == nil && user != nil {
			avatar = user.Avatar
		}
	}

	ackEvent := protocol.MessageReadAckEvent{
		ConversationId: conversationId,
		LastReadSeq:    lastReadSeq,
		UserId:         userId,
		ConvType:       protocol.ConvType(convType),
		SenderId:       senderId,
		Avatar:         avatar,
	}

	if ma.taskManager != nil {
		_ = ma.taskManager.PublishMessageReadAck(
			ctx,
			protocol.EventMessageReadAck,
			userId+":"+conversationId,
			ackEvent,
		)
	}
}

func (ma *MessageApplication) CheckRoomMember(ctx context.Context, userId string, roomId string) error {
	ok, err := ma.checkConvMember(ctx, messagevo.RoomChat, roomId, userId, roomId)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotRoomMember
	}
	return nil
}

func (ma *MessageApplication) GetRoomMemberIDs(ctx context.Context, roomId string) ([]string, error) {
	members, cacheVersion, err := ma.conversationCache.GetMembersWithVersion(ctx, roomId)
	if err == nil && len(members) > 0 {
		room, roomErr := ma.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate))
		if roomErr == nil && cacheVersion == room.Version {
			return members, nil
		}
	}

	room, err := ma.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate))
	if err != nil {
		return nil, err
	}

	members, err = ma.roomUserRepository.ListActiveUserIDs(roomId)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, ErrNotRoomMember
	}
	if cacheErr := ma.conversationCache.SetMembers(ctx, roomId, members, room.Version); cacheErr != nil {
		// best-effort cache warmup; the DB state is already authoritative
	}
	return members, nil
}

func (ma *MessageApplication) buildMediaWriter(dto *MessageAppeDTO, messageId string) (func(context.Context, *gorm.DB) error, error) {
	if dto == nil {
		return nil, nil
	}

	switch messagevo.CType(dto.CType) {
	case messagevo.Image:
		if ma.messageImageRepository == nil {
			return nil, fmt.Errorf("message image repository is not configured")
		}
		if dto.FileId == "" && dto.MediaURL == "" {
			return nil, fmt.Errorf("image media is required")
		}
		item := messageentity.NewMessageImage(
			messageId,
			dto.FileId,
			dto.ThumbFileId,
			"",
			dto.MediaURL,
			dto.Width,
			dto.Height,
			dto.FileSize,
		)
		return func(ctx context.Context, tx *gorm.DB) error {
			return ma.messageImageRepository.WithTx(tx).Create(ctx, item)
		}, nil
	case messagevo.File:
		if ma.messageFileRepository == nil {
			return nil, fmt.Errorf("message file repository is not configured")
		}
		if dto.FileId == "" && dto.MediaURL == "" {
			return nil, fmt.Errorf("file media is required")
		}
		item := messageentity.NewMessageFile(
			messageId,
			dto.FileId,
			dto.FileName,
			dto.FileName,
			"",
			dto.MediaURL,
			dto.FileSize,
		)
		return func(ctx context.Context, tx *gorm.DB) error {
			return ma.messageFileRepository.WithTx(tx).Create(ctx, item)
		}, nil
	case messagevo.Sticker:
		if ma.messageStickerRepository == nil {
			return nil, fmt.Errorf("message sticker repository is not configured")
		}
		if dto.StickerId == "" && dto.MediaURL == "" {
			return nil, fmt.Errorf("sticker media is required")
		}
		item := messageentity.NewMessageSticker(
			messageId,
			dto.StickerId,
			dto.PackId,
			dto.MediaURL,
			dto.Width,
			dto.Height,
		)
		return func(ctx context.Context, tx *gorm.DB) error {
			return ma.messageStickerRepository.WithTx(tx).Create(ctx, item)
		}, nil
	case messagevo.Video:
		if ma.messageVideoRepository == nil {
			return nil, fmt.Errorf("message video repository is not configured")
		}
		if dto.FileId == "" && dto.MediaURL == "" {
			return nil, fmt.Errorf("video media is required")
		}
		duration := int64(0)
		if dto.DurationMs != nil {
			duration = *dto.DurationMs
		}
		item := messageentity.NewMessageVideo(
			messageId,
			dto.FileId,
			dto.ThumbFileId,
			dto.MediaURL,
			duration,
			dto.Width,
			dto.Height,
		)
		return func(ctx context.Context, tx *gorm.DB) error {
			return ma.messageVideoRepository.WithTx(tx).Create(ctx, item)
		}, nil
	default:
		return nil, nil
	}
}

func (ma *MessageApplication) HandleMessage(ctx context.Context, dto MessageAppeDTO) (*MessageAppeDTO, error) {
	conversationId := messageentity.GetConversationID(dto.SendId, dto.RecvId, dto.ConvType)
	messageId, err := snow.GenerateSnowID(int(ma.config.App.MachineID))
	if err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	// 幂等检查：同一 clientMsgId 在 5 分钟内只处理一次
	if dto.ClientMsgId != "" && ma.conversationCache != nil {
		if existingMsgId, err := ma.conversationCache.GetDedupEntry(ctx, dto.ClientMsgId); err == nil && existingMsgId != "" {
			return &MessageAppeDTO{
				ClientMsgId: dto.ClientMsgId,
				MessageId:   existingMsgId,
				Status:      string(protocol.AckStatusSent),
			}, nil
		}
	}

	ok, err := ma.checkConvMember(
		ctx,
		messagevo.ConvType(dto.ConvType),
		conversationId,
		dto.SendId,
		dto.RecvId,
	)

	if !ok {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			MessageId:   messageId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	if err != nil {
		log.Println("failed to add member in conv cache ", err)
	}

	seq, err := ma.conversationCache.IncrConvLatestSeq(ctx, conversationId)

	if err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			MessageId:   messageId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	messageType := messagevo.CType(dto.CType)
	message := messageentity.NewMessage(
		messageId,
		conversationId,
		dto.SendId,
		seq,
		messageType,
		dto.Content,
		dto.VideoId,
		dto.VideoTime,
	)

	mediaWriter, err := ma.buildMediaWriter(&dto, messageId)
	if err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			MessageId:   messageId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	conv := messageentity.NewConversation(
		conversationId,
		dto.SendId,
		dto.RecvId,
		dto.ConvType,
		seq,
		messageId,
	)

	userConv := messageentity.BuildUserConversation(
		dto.SendId,
		conversationId,
		seq,
		seq,
	)

	isDanmaku := dto.ConvType == int(messagevo.RoomChat) && dto.VideoTime != nil
	if ma.txManager == nil || ma.messageOutboxRepository == nil {
		return nil, fmt.Errorf("message outbox is not configured")
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
		Seq:            seq,
		ConvType:       protocol.ConvType(dto.ConvType),
		CType:          dto.CType,
		Content:        dto.Content,
		SendTime:       message.SendTime,
		ClientMsgId:    dto.ClientMsgId,
		MediaURL:       dto.MediaURL,
		ThumbURL:       dto.ThumbURL,
		FileId:         dto.FileId,
		ThumbFileId:    dto.ThumbFileId,
		FileName:       dto.FileName,
		FileSize:       dto.FileSize,
		Width:          dto.Width,
		Height:         dto.Height,
		DurationMs:     dto.DurationMs,
		StickerId:      dto.StickerId,
		PackId:         dto.PackId,
		HasVideoTime:   dto.VideoTime != nil,
		VideoTime:      dto.VideoTime,
	}
	eventPayload, err := json.Marshal(messageEvent)
	if err != nil {
		return nil, err
	}

	// TODO： 这里直接操作了数据库，几乎一条消息就会进行一次处理，解决：这一步写入到 kafka 中，前端根据是否接收到 ack，继而重试（保持 clientMessageId 不变）
	err = ma.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		msgRepo := ma.messageRepository.WithTx(tx)
		convRepo := ma.conversationRepository.WithTx(tx)
		userConvRepo := ma.userConversationRepository.WithTx(tx)
		outboxRepo := ma.messageOutboxRepository.WithTx(tx)

		if err := msgRepo.Save(ctx, message); err != nil {
			// write failure is fatal; outbox only covers the downstream MQ dispatch
			return ErrMessageSave
		}

		if !isDanmaku {
			if err = convRepo.Upsert(ctx, conv); err != nil {
				return ErrConversationSequenceUpdate
			}

			if err := userConvRepo.UpdateReadSeq(ctx, userConv); err != nil {
				log.Printf("warn: update sender uc failed: %v", err)
			}

			if err := userConvRepo.UpdateSyncSeq(ctx, userConv); err != nil {
				log.Printf("warn: update sender uc failed: %v", err)
			}
		}

		if mediaWriter != nil {
			if err := mediaWriter(ctx, tx); err != nil {
				return err
			}
		}

		outbox := &messageentity.MessageOutbox{
			EventType:  string(protocol.EventTypeMessage),
			Topic:      string(protocol.EventTypeMessage),
			MessageKey: conversationId,
			Payload:    eventPayload,
		}
		if err := outboxRepo.Create(ctx, outbox); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return &MessageAppeDTO{
			ClientMsgId: dto.ClientMsgId,
			MessageId:   messageId,
			Status:      string(protocol.AckStatusFailed),
		}, err
	}

	// 标记已处理，5 分钟内同一 clientMsgId 幂等返回
	if dto.ClientMsgId != "" && ma.conversationCache != nil {
		_, _ = ma.conversationCache.SetDedupEntry(ctx, dto.ClientMsgId, messageId, 5*time.Minute)
	}

	return &MessageAppeDTO{
		ClientMsgId:    dto.ClientMsgId,
		MessageId:      messageId,
		SenderUsername: senderUsername,
		Status:         string(protocol.AckStatusSent),
	}, nil
}

func (ma *MessageApplication) GetHistoryMessages(
	ctx context.Context,
	conversationId string,
	userId string,
	limit int,
	cursor int64,
) ([]MessageAppeDTO, int64, bool, error) {

	// 限制 limit 大小
	if limit < 0 || limit > 50 {
		limit = 20
	}

	var maxSeq int64
	switch cursor {
	case 0:
		// 首次查询
		uc, err := ma.userConversationRepository.GetUserConversation(ctx, userId, conversationId)
		if err != nil || uc == nil {
			return nil, -1, false, ErrConversationNotFound
		}

		if uc.LastReadSeq > 0 {
			// 之前读过，从未读开始
			maxSeq = uc.LastReadSeq + 1
		} else {
			// 从未读过，直接从最新的开始读
			maxSeq = math.MaxInt64
		}
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
		limit,
	)
	if err != nil {
		return nil, -1, false, err
	}

	msgsApp := toMessagesAppDTO(msgs)
	ma.fillSenderUsernames(msgsApp)
	ma.fillMediaFields(ctx, msgsApp)

	sort.Slice(msgsApp, func(i, j int) bool {
		return msgsApp[i].Seq < msgsApp[j].Seq
	})

	var nextCursor int64 = 0
	var hasMore bool = true

	if len(msgsApp) > 0 && msgsApp[0].Seq != 1 {
		nextCursor = msgsApp[0].Seq
		hasMore = true
	} else {
		hasMore = false
		nextCursor = -1
	}

	return msgsApp, nextCursor, hasMore, nil
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

func (ma *MessageApplication) fillSenderUsernames(messages []MessageAppeDTO) {
	if len(messages) == 0 || ma.userRepository == nil {
		return
	}
	seen := make(map[string]struct{}, len(messages))
	userIds := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.SendId == "" {
			continue
		}
		if _, ok := seen[msg.SendId]; ok {
			continue
		}
		seen[msg.SendId] = struct{}{}
		userIds = append(userIds, msg.SendId)
	}
	users, err := ma.userRepository.FindByUserIDs(userIds)
	if err != nil {
		return
	}
	usernameById := make(map[string]string, len(users))
	for _, user := range users {
		usernameById[user.UserId] = user.UserName
	}
	for i := range messages {
		messages[i].SenderUsername = usernameById[messages[i].SendId]
	}
}

// fillMediaFields batch-fetches media sub-entities (image/video/file/sticker)
// and merges their fields into the DTOs. This replaces the old pattern of storing
// media fields directly on the messages table.
func (ma *MessageApplication) fillMediaFields(ctx context.Context, messages []MessageAppeDTO) {
	if len(messages) == 0 {
		return
	}

	// Collect message IDs grouped by type
	imageIDs := make([]string, 0)
	videoIDs := make([]string, 0)
	fileIDs := make([]string, 0)
	stickerIDs := make([]string, 0)

	for _, m := range messages {
		switch messagevo.CType(m.CType) {
		case messagevo.Image:
			imageIDs = append(imageIDs, m.MessageId)
		case messagevo.Video:
			videoIDs = append(videoIDs, m.MessageId)
		case messagevo.File:
			fileIDs = append(fileIDs, m.MessageId)
		case messagevo.Sticker:
			stickerIDs = append(stickerIDs, m.MessageId)
		}
	}

	// Batch-fetch from sub-repositories
	var (
		images   map[string]*messageentity.MessageImage
		videos   map[string]*messageentity.MessageVideo
		files    map[string]*messageentity.MessageFile
		stickers map[string]*messageentity.MessageSticker
	)

	if len(imageIDs) > 0 && ma.messageImageRepository != nil {
		images, _ = ma.messageImageRepository.BatchGetByMessageIDs(ctx, imageIDs)
	}
	if len(videoIDs) > 0 && ma.messageVideoRepository != nil {
		videos, _ = ma.messageVideoRepository.BatchGetByMessageIDs(ctx, videoIDs)
	}
	if len(fileIDs) > 0 && ma.messageFileRepository != nil {
		files, _ = ma.messageFileRepository.BatchGetByMessageIDs(ctx, fileIDs)
	}
	if len(stickerIDs) > 0 && ma.messageStickerRepository != nil {
		stickers, _ = ma.messageStickerRepository.BatchGetByMessageIDs(ctx, stickerIDs)
	}

	// Populate DTO fields from sub-entities
	for i := range messages {
		switch messagevo.CType(messages[i].CType) {
		case messagevo.Image:
			if img, ok := images[messages[i].MessageId]; ok && img != nil {
				messages[i].FileId = img.FileId
				messages[i].ThumbFileId = img.ThumbFileId
				messages[i].Width = img.Width
				messages[i].Height = img.Height
				messages[i].FileSize = img.Size
				messages[i].MediaURL = img.URL
			}
		case messagevo.Video:
			if vid, ok := videos[messages[i].MessageId]; ok && vid != nil {
				messages[i].FileId = vid.FileId
				messages[i].ThumbFileId = vid.CoverFileId
				messages[i].Width = vid.Width
				messages[i].Height = vid.Height
				messages[i].DurationMs = &vid.DurationMs
				messages[i].MediaURL = vid.URL
			}
		case messagevo.File:
			if f, ok := files[messages[i].MessageId]; ok && f != nil {
				messages[i].FileId = f.FileId
				messages[i].FileName = f.FileName
				messages[i].FileSize = f.Size
				messages[i].MediaURL = f.URL
			}
		case messagevo.Sticker:
			if s, ok := stickers[messages[i].MessageId]; ok && s != nil {
				messages[i].StickerId = s.StickerId
				messages[i].PackId = s.PackId
				messages[i].Width = s.Width
				messages[i].Height = s.Height
				messages[i].MediaURL = s.URL
			}
		}
	}
}

func (ma *MessageApplication) fillConversationDisplayNames(
	messages []MessageAppeDTO,
	conversations []*messageentity.Conversation,
	userId string,
) {
	if len(messages) == 0 || len(conversations) == 0 {
		return
	}

	conversationById := make(map[string]*messageentity.Conversation, len(conversations))
	privatePeerIds := make([]string, 0, len(conversations))
	privateSeen := make(map[string]struct{}, len(conversations))
	roomIds := make([]string, 0, len(conversations))
	roomSeen := make(map[string]struct{}, len(conversations))

	for _, conv := range conversations {
		if conv == nil {
			continue
		}
		conversationById[conv.ConversationId] = conv

		switch conv.Convtype {
		case messagevo.PrivateChat:
			peerId := conv.UserId1
			if peerId == userId {
				peerId = conv.UserId2
			}
			if peerId != "" {
				if _, ok := privateSeen[peerId]; !ok {
					privateSeen[peerId] = struct{}{}
					privatePeerIds = append(privatePeerIds, peerId)
				}
			}
		case messagevo.RoomChat:
			if conv.RoomId != "" {
				if _, ok := roomSeen[conv.RoomId]; !ok {
					roomSeen[conv.RoomId] = struct{}{}
					roomIds = append(roomIds, conv.RoomId)
				}
			}
		}
	}

	userMetaByID := make(map[string]userentity.User, len(privatePeerIds))
	if len(privatePeerIds) > 0 && ma.userRepository != nil {
		if users, err := ma.userRepository.FindByUserIDs(privatePeerIds); err == nil {
			for _, user := range users {
				userMetaByID[user.UserId] = user
			}
		}
	}

	type roomMeta struct {
		displayName string
		avatar      string
	}
	roomMetaByID := make(map[string]roomMeta, len(roomIds))
	if len(roomIds) > 0 && ma.roomRepository != nil {
		for _, roomID := range roomIds {
			room, err := ma.roomRepository.FindActiveRoom(roomID, int(roomvo.Activate))
			if err != nil || room == nil {
				continue
			}
			roomMetaByID[roomID] = roomMeta{displayName: room.RoomName, avatar: room.Avatar}
		}
	}

	for i := range messages {
		conv := conversationById[messages[i].ConversationID]
		if conv == nil {
			continue
		}

		messages[i].ConvType = int(conv.Convtype)
		switch conv.Convtype {
		case messagevo.PrivateChat:
			peerId := conv.UserId1
			if peerId == userId {
				peerId = conv.UserId2
			}
			if user, ok := userMetaByID[peerId]; ok {
				if user.NickName != "" {
					messages[i].DisplayName = user.NickName
				} else {
					messages[i].DisplayName = user.UserName
				}
				messages[i].Avatar = user.Avatar
				continue
			}
			messages[i].DisplayName = ma.getUserDisplayName(peerId)
			messages[i].Avatar = ma.getUserAvatar(peerId)
		case messagevo.RoomChat:
			if meta, ok := roomMetaByID[conv.RoomId]; ok {
				messages[i].DisplayName = meta.displayName
				messages[i].Avatar = meta.avatar
				continue
			}
			messages[i].DisplayName = ma.getRoomDisplayName(conv.RoomId)
			messages[i].Avatar = ma.getRoomAvatar(conv.RoomId)
		}
	}
}

func (ma *MessageApplication) getUserDisplayName(userId string) string {
	if userId == "" || ma.userRepository == nil {
		return ""
	}
	user, err := ma.userRepository.FindByUserID(userId)
	if err != nil || user == nil {
		return ""
	}
	if user.NickName != "" {
		return user.NickName
	}
	return user.UserName
}

func (ma *MessageApplication) getUserAvatar(userId string) string {
	if userId == "" || ma.userRepository == nil {
		return ""
	}
	user, err := ma.userRepository.FindByUserID(userId)
	if err != nil || user == nil {
		return ""
	}
	return user.Avatar
}

func (ma *MessageApplication) getRoomDisplayName(roomId string) string {
	if roomId == "" || ma.roomRepository == nil {
		return ""
	}
	room, err := ma.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate))
	if err != nil || room == nil {
		return ""
	}
	return room.RoomName
}

func (ma *MessageApplication) getRoomAvatar(roomId string) string {
	if roomId == "" || ma.roomRepository == nil {
		return ""
	}
	room, err := ma.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate))
	if err != nil || room == nil {
		return ""
	}
	return room.Avatar
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
	if err := ma.CheckRoomMember(ctx, userId, roomId); err != nil {
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
	if err := ma.CheckRoomMember(ctx, userId, roomId); err != nil {
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

func (ma *MessageApplication) GetOfflineMessages(
	ctx context.Context,
	userId string,
) ([]MessageAppeDTO, map[string]int64, error) {
	uconvs, err := ma.userConversationRepository.ListByUser(ctx, userId)
	if err != nil {
		return nil, nil, ErrConversationNotFound
	}

	readMap := make(map[string]int64)
	convIds := make([]string, 0, len(uconvs))
	for _, uc := range uconvs {
		readMap[uc.ConversationId] = uc.LastReadSeq
		convIds = append(convIds, uc.ConversationId)
	}

	convs, err := ma.conversationRepository.ListByIDs(ctx, convIds)
	if err != nil {
		return nil, nil, ErrConversationNotFound
	}

	syncMap := make(map[string]int64)
	unreadMap := make(map[string]int64)

	for _, c := range convs {
		unreadMap[c.ConversationId] = c.LatestSeq - readMap[c.ConversationId]
		syncMap[c.ConversationId] = c.LatestSeq
	}

	msgs, err := ma.messageRepository.GetLatestMessagesByConversationIDs(
		ctx,
		convIds,
	)

	if err != nil {
		return nil, nil, err
	}

	syncItems := make([]*messageentity.UserConversation, 0, len(uconvs))

	for _, uconv := range uconvs {
		latestSeq := syncMap[uconv.ConversationId]
		if latestSeq == uconv.LatestSyncSeq {
			continue
		}

		syncItems = append(syncItems, messageentity.BuildUserConversation(
			uconv.UserId,
			uconv.ConversationId,
			0,
			latestSeq,
		))
	}

	if len(syncItems) > 0 {
		if err := ma.userConversationRepository.BatchUpdateSyncSeq(ctx, syncItems); err != nil {
			log.Printf("warn: batch update sync seq failed: %v", err)
		}
	}

	msgsApp := toMessagesAppDTO(msgs)
	ma.fillSenderUsernames(msgsApp)
	ma.fillMediaFields(ctx, msgsApp)
	ma.fillConversationDisplayNames(msgsApp, convs, userId)
	return msgsApp, unreadMap, nil
}
