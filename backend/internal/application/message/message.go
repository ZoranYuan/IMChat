package message

import (
	"IM_backend/configs"
	mqport "IM_backend/internal/application/ports/mq"
	convcache "IM_backend/internal/application/ports/persistence/cache/conversation"
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/id/snow"
	"IM_backend/internal/infrastructure/persistence/redis/cache/local"
	"IM_backend/internal/shared/protocol"
	"context"
	"fmt"
	"log"
	"math"
	"sort"

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
	roomUserRepository         roomrepo.RoomUserRepository
	roomRepository             roomrepo.RoomRepository
	taskManager                mqport.TaskManager
	localConvVersionCache      *local.ConversationVersionCache
	sf                         singleflight.Group
}

func NewMessageApplication(
	config configs.Config,
	conversationCache convcache.ConversationCache,
	txManager txmanager.TxManager,
	taskManager mqport.TaskManager,
	userConversationRepository messagerepo.UserConversationRepository,
	conversationRepository messagerepo.ConversationRepository,
	friendRepository friendrepo.FriendRepository,
	messageRepository messagerepo.MessageRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	roomRepository roomrepo.RoomRepository,
	localConvVersionCache *local.ConversationVersionCache,
) *MessageApplication {
	return &MessageApplication{
		config:                     config,
		conversationCache:          conversationCache,
		txManager:                  txManager,
		taskManager:                taskManager,
		messageRepository:          messageRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
		roomRepository:             roomRepository,
		roomUserRepository:         roomUserRepository,
		localConvVersionCache:      localConvVersionCache,
		friendRepository:           friendRepository,
	}
}

func (ma *MessageApplication) checkConvMember(
	ctx context.Context,
	convType messagevo.ConvType,
	conversationId string,
	userId string,
	recvId string,
) (bool, error) {
	localVer, ok := ma.localConvVersionCache.GetVersion(conversationId)

	isMember, cacheVersion, err := ma.conversationCache.IsMemberWithVersion(
		ctx, conversationId, userId,
	)
	if err == nil {
		if ok && localVer == cacheVersion {
			return isMember, nil
		}
	}

	key := userId + ":" + conversationId

	v, err, _ := ma.sf.Do(key, func() (any, error) {
		localVer, ok := ma.localConvVersionCache.GetVersion(conversationId)

		isMember, cacheVersion, err := ma.conversationCache.IsMemberWithVersion(
			ctx, conversationId, userId,
		)
		if err == nil {
			if ok && localVer == cacheVersion {
				return isMember, nil
			}
		}

		var cacheErr error

		if convType == messagevo.PrivateChat {
			friend, err := ma.friendRepository.FindRelation(userId, recvId)
			if err != nil {
				return false, err
			}
			if friend == nil {
				return false, ErrNotFriends
			}
			cacheErr = ma.conversationCache.SetMembers(
				ctx,
				conversationId,
				[]string{userId, recvId},
				cacheVersion,
			)
		} else {
			room, err := ma.roomRepository.FindActiveRoom(recvId, int(roomvo.Activate))
			if err != nil {
				return false, err
			}
			roomUser, err := ma.roomUserRepository.GetRelationByIDs(userId, recvId)
			if err != nil {
				return false, err
			}
			if roomUser == nil {
				return false, ErrNotRoomMember
			}

			cacheErr = ma.conversationCache.AddMember(
				ctx,
				conversationId,
				userId,
				room.Version,
			)
		}

		ma.localConvVersionCache.SetVersion(conversationId, cacheVersion)

		return true, cacheErr
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
) error {
	uconv, err := ma.userConversationRepository.GetUserConversation(ctx, userId, conversationId)

	if err != nil {
		return ErrConversationNotFound
	}

	if lastReadSeq <= uconv.LastReadSeq {
		// TODO: 如何处理之前的应答消息
		return nil
	}

	uconv.UpdateReadSeq(lastReadSeq)

	err = ma.userConversationRepository.UpdateReadSeq(
		ctx,
		uconv,
	)

	if err != nil {
		return err
	}

	go func() {
		event := protocol.MessageReadAckEvent{
			ConversationId: conversationId,
			LastReadSeq:    lastReadSeq,
			UserId:         userId,
		}

		ma.taskManager.SendHistoryMessageAck(
			ctx,
			protocol.EventMessageReadAck,
			userId+":"+conversationId,
			event,
		)
	}()

	return nil
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
	members, _, err := ma.conversationCache.GetMembersWithVersion(ctx, roomId)
	if err == nil && len(members) > 0 {
		return members, nil
	}

	members, err = ma.roomUserRepository.ListActiveUserIDs(roomId)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, ErrNotRoomMember
	}
	return members, nil
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

	message := messageentity.BuildMessage(
		messageId,
		conversationId,
		dto.SendId,
		seq,
		dto.Content,
		dto.VideoTime,
		messagevo.CType(dto.CType),
	)

	conv := messageentity.BuildConversation(
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

	err = ma.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		msgRepo := ma.messageRepository.WithTx(tx)
		convRepo := ma.conversationRepository.WithTx(tx)
		userConvRepo := ma.userConversationRepository.WithTx(tx)

		if err := msgRepo.Save(ctx, message); err != nil {
			// TODO 异步补偿机制
			return ErrMessageSave
		}

		if err = convRepo.Upsert(ctx, conv); err != nil {
			return ErrConversationSequenceUpdate
		}

		if err := userConvRepo.UpdateReadSeq(ctx, userConv); err != nil {
			log.Printf("warn: update sender uc failed: %v", err)
		}

		if err := userConvRepo.UpdateSyncSeq(ctx, userConv); err != nil {
			log.Printf("warn: update sender uc failed: %v", err)
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

	value := ctx.Value("op")
	op, ok := value.(string)
	if !ok || op == "" {
		return nil, ErrUnknown
	}

	messageEvent := protocol.MessageEvent{
		MessageId:      messageId,
		ConversationId: conversationId,
		SendId:         dto.SendId,
		RecvId:         dto.RecvId,
		Seq:            seq,
		ConvType:       protocol.ConvType(dto.ConvType),
		CType:          dto.CType,
		Content:        dto.Content,
		SendTime:       message.SendTime,
	}

	go func() {
		if err := ma.taskManager.SendMessage(
			context.Background(),
			protocol.EventTypeMessage,
			conversationId,
			messageEvent,
		); err != nil {
			log.Printf("mq send failed: %v", err)
			// TODO: 后续做补偿（Outbox）
		}
	}()

	return &MessageAppeDTO{
		ClientMsgId: dto.ClientMsgId,
		MessageId:   messageId,
		Status:      string(protocol.AckStatusSent),
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
			// TODO: 当前会话找不到，解决办法是什么？
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

func (ma *MessageApplication) GetRoomDanmaku(
	ctx context.Context,
	roomId string,
	userId string,
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

	msgs, err := ma.messageRepository.GetMessagesBySendTime(ctx, roomId, startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return []DanmakuDTO{}, nil
	}

	baseTime := startTime
	if baseTime == 0 {
		baseTime = msgs[0].SendTime
	}

	res := make([]DanmakuDTO, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil || msg.Type != messagevo.Text {
			continue
		}

		offset := msg.SendTime - baseTime
		if offset < 0 {
			offset = 0
		}
		res = append(res, DanmakuDTO{
			MessageId: msg.MessageId,
			SenderId:  msg.SendId,
			Content:   msg.Content,
			Seq:       msg.Seq,
			TimeMs:    offset,
			SendTime:  msg.SendTime,
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

	syncItems := make([]protocol.ConversationSyncSeqItem, 0, len(uconvs))

	for _, uconv := range uconvs {
		latestSeq := syncMap[uconv.ConversationId]
		if latestSeq == uconv.LatestSyncSeq {
			// 当前会话的最大值不需要再更新了
			continue
		}

		syncItems = append(syncItems, protocol.ConversationSyncSeqItem{
			UserId:         uconv.UserId,
			ConversationId: uconv.ConversationId,
			LatestSeq:      latestSeq,
		})
	}

	if len(syncItems) > 0 {
		go func(items []protocol.ConversationSyncSeqItem) {
			event := protocol.ConversationSyncSeqEvent{Items: items}
			if err := ma.taskManager.SendConversationSyncSeq(
				context.Background(),
				protocol.EventConversationSyncSeq,
				userId,
				event,
			); err != nil {
				log.Printf("mq send conversation sync seq failed: %v", err)
			}
		}(syncItems)
	}

	return toMessagesAppDTO(msgs), unreadMap, nil
}
