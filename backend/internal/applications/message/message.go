package application_message

import (
	"IM_backend/configs"
	mq_interface "IM_backend/internal/applications/interface/mq"
	tx_repository_interface "IM_backend/internal/applications/interface/repository/tx_manager"
	message_entity "IM_backend/internal/domain/message/entity"
	message_repository_interface "IM_backend/internal/domain/message/repository"
	message_valueobject "IM_backend/internal/domain/message/value_object"
	"IM_backend/internal/infrastructure/pkg/snow"
	conversation_port "IM_backend/internal/port/conversation"
	"IM_backend/internal/protocol"
	"context"
	"log"
	"math"
	"sort"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

type MessageApplication struct {
	config                     configs.Config
	conversationCache          conversation_port.ConversationCacheInterface
	messageRepository          message_repository_interface.MessageRepositoryInterface
	txManager                  tx_repository_interface.TxRepositoryInterface
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface
	conversationRepository     message_repository_interface.ConversationRepositoryInterface
	taskManager                mq_interface.TaskManager
	sf                         singleflight.Group
}

func NewMessageApplication(
	config configs.Config,
	conversationCache conversation_port.ConversationCacheInterface,
	txManager tx_repository_interface.TxRepositoryInterface,
	taskManager mq_interface.TaskManager,
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface,
	conversationRepository message_repository_interface.ConversationRepositoryInterface,
	messageRepository message_repository_interface.MessageRepositoryInterface,
) *MessageApplication {
	return &MessageApplication{
		config:                     config,
		conversationCache:          conversationCache,
		txManager:                  txManager,
		taskManager:                taskManager,
		messageRepository:          messageRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
	}
}

func (ma *MessageApplication) checkConvMember(
	ctx context.Context,
	convType message_valueobject.ConvType,
	conversationId string,
	userId string,
) (bool, error) {

	isMember, err := ma.conversationCache.IsMember(ctx, conversationId, userId)
	if err != nil {
		return false, err
	}

	if isMember {
		return true, nil
	}

	v, err, _ := ma.sf.Do(conversationId, func() (any, error) {
		ok, err := ma.conversationCache.IsMember(ctx, conversationId, userId)
		if err != nil {
			return false, err
		}

		if !ok {
			if convType == message_valueobject.PrivateChat {
				return false, ErrNotFriend
			} else {
				return false, ErrNotInRoom
			}
		}

		err = ma.conversationCache.AddMember(ctx, conversationId, userId)
		return true, err
	})

	return v.(bool), err
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

func (ma *MessageApplication) HandleMessage(ctx context.Context, dto MessageAppeDTO) (*MessageAppeDTO, error) {
	conversationId := message_entity.GetConversationId(dto.SendId, dto.RecvId, dto.ConvType)
	messageId, err := snow.GenerateSnowId(int(ma.config.App.MachineID))

	ok, err := ma.checkConvMember(
		ctx,
		message_valueobject.ConvType(dto.ConvType),
		conversationId,
		dto.SendId,
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

	message := message_entity.BuildMessage(
		messageId,
		conversationId,
		dto.SendId,
		seq,
		dto.Content,
		dto.VideoTime,
		message_valueobject.CType(dto.CType),
	)

	conv := message_entity.BuildConversation(
		conversationId,
		dto.SendId,
		dto.RecvId,
		dto.ConvType,
		seq,
		messageId,
	)

	userConv := message_entity.BuildUserConversation(
		dto.SendId,
		conversationId,
		messageId,
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

		if err := convRepo.Upsert(ctx, conv); err != nil {
			return ErrConversationUpdateSeq
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

	convs, err := ma.conversationRepository.ListByIds(ctx, convIds)
	if err != nil {
		return nil, nil, ErrConversationNotFound
	}

	syncMap := make(map[string]int64)
	unreadMap := make(map[string]int64)

	for _, c := range convs {
		unreadMap[c.ConversationId] = c.LatestSeq - readMap[c.ConversationId]
		syncMap[c.ConversationId] = c.LatestSeq
	}

	msgs, err := ma.messageRepository.GetLatestMessageByConv(
		ctx,
		convIds,
	)

	if err != nil {
		return nil, nil, err
	}

	type syncJob struct {
		userConv  *message_entity.UserConversation
		latestSeq int64
	}

	jobs := make([]syncJob, 0, len(uconvs))

	for _, uconv := range uconvs {
		latestSeq := syncMap[uconv.ConversationId]
		if latestSeq == uconv.LatestSyncSeq {
			// 当前会话的最大值不需要再更新了
			continue
		}

		jobs = append(jobs, syncJob{
			userConv:  uconv,
			latestSeq: latestSeq,
		})
	}

	if len(jobs) > 0 {
		go func(jobs []syncJob) {
			for _, job := range jobs {
				job.userConv.UpdateSyncSeq(job.latestSeq)
				if err := ma.userConversationRepository.UpdateSyncSeq(
					// 这里必须要额外一个 ctx
					context.Background(),
					job.userConv,
				); err != nil {
					log.Printf("marn: async update sync seq failed: %v", err)
				}
			}
		}(jobs)
	}

	return toMessagesAppDTO(msgs), unreadMap, nil
}
