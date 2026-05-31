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
	"IM_backend/internal/infrastructure/id/snow"
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
	fileRepository             filerepo.FileRepository
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
	friendRepository friendrepo.FriendRepository,
	fileRepository filerepo.FileRepository,
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
		messageRepository:          messageRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
		userRepository:             userRepository,
		roomRepository:             roomRepository,
		roomUserRepository:         roomUserRepository,
		friendRepository:           friendRepository,
		fileRepository:             fileRepository,
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

		if err == nil && cacheVersion > 0 && cacheVersion == room.Version {
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
		for _, memberID := range members {
			if memberID == userId {
				return true, nil
			}
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
		// 缓存回填失败不阻断主流程
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
		dto.VideoId,
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
	ma.fillSenderUsernames(msgsApp)

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

func (ma *MessageApplication) fillConversationDisplayNames(
	messages []MessageAppeDTO,
	conversations []*messageentity.Conversation,
	userId string,
) {
	if len(messages) == 0 || len(conversations) == 0 {
		return
	}

	conversationById := make(map[string]*messageentity.Conversation, len(conversations))
	for _, conv := range conversations {
		if conv == nil {
			continue
		}
		conversationById[conv.ConversationId] = conv
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
			messages[i].DisplayName = ma.getUserDisplayName(peerId)
		case messagevo.RoomChat:
			messages[i].DisplayName = ma.getRoomDisplayName(conv.RoomId)
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

	msgsApp := toMessagesAppDTO(msgs)
	ma.fillSenderUsernames(msgsApp)
	ma.fillConversationDisplayNames(msgsApp, convs, userId)
	return msgsApp, unreadMap, nil
}
