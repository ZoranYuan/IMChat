package application_message

import (
	"IM_backend/configs"
	conversation_cache_interface "IM_backend/internal/applications/interface/cache/conversation"
	message_cache_interface "IM_backend/internal/applications/interface/cache/message"
	mq_interface "IM_backend/internal/applications/interface/mq"
	tx_repository_interface "IM_backend/internal/applications/interface/repository/tx_manager"
	message_entity "IM_backend/internal/domain/message/entity"
	message_repository_interface "IM_backend/internal/domain/message/repository"
	message_valueobject "IM_backend/internal/domain/message/value_object"
	"IM_backend/internal/infrastructure/pkg/snow"
	"IM_backend/internal/protocol"
	"context"
	"errors"
	"log"
	"math"
	"sort"

	"gorm.io/gorm"
)

type MessageApplication struct {
	config                     configs.Config
	messageCache               message_cache_interface.MessageCacheInterface
	conversationCache          conversation_cache_interface.ConversationCacheInterface
	messageRepository          message_repository_interface.MessageRepositoryInterface
	txManager                  tx_repository_interface.TxRepositoryInterface
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface
	conversationRepository     message_repository_interface.ConversationRepositoryInterface
	taskManager                mq_interface.TaskManager
}

func NewMessageApplication(
	config configs.Config,
	messageCache message_cache_interface.MessageCacheInterface,
	conversationCache conversation_cache_interface.ConversationCacheInterface,
	txManager tx_repository_interface.TxRepositoryInterface,
	taskManager mq_interface.TaskManager,
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface,
	conversationRepository message_repository_interface.ConversationRepositoryInterface,
	messageRepository message_repository_interface.MessageRepositoryInterface,
) *MessageApplication {
	return &MessageApplication{
		config:                     config,
		messageCache:               messageCache,
		conversationCache:          conversationCache,
		txManager:                  txManager,
		taskManager:                taskManager,
		messageRepository:          messageRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
	}
}

func (ma *MessageApplication) checkMember(
	ctx context.Context,
	userId string,
	convId string,
) error {

	isMember, err := ma.conversationCache.IsMember(ctx, convId, userId)

	if err == nil && isMember {
		return nil
	}

	// // cache miss / error → DB fallback
	// isMember, err = ma.conversationCache.IsMember(ctx, convId, userId)
	// if err != nil {
	// 	return ErrUnknown
	// }

	// if !isMember {
	// 	return ErrForbidden
	// }

	// // 回填 cache
	// _ = ma.conversationCache.AddMember(ctx, convId, userId)

	return nil
}

func (ma *MessageApplication) HandleHistoryMessageReadAck(
	ctx context.Context,
	userId string,
	conversationId string,
	lastReadSeq int64,
) error {
	uconv, err := ma.userConversationRepository.GetUserConversation(ctx, userId, conversationId)

	if err != nil {
		return ErrConversationNotFound
	}

	uconv.UpdateReadSeq(lastReadSeq)

	err = ma.userConversationRepository.UpdateReadSeq(
		ctx,
		uconv,
	)
	if err != nil {
		return err
	}

	event := protocol.HistoryMessageReadAckEvent{
		ConversationId: conversationId,
		LastReadSeq:    lastReadSeq,
		To:             userId,
	}

	ma.taskManager.SendHistoryMessageAck(
		ctx,
		protocol.EventTypeHistoryMessageReadAck,
		userId+":"+conversationId,
		event,
	)

	return nil
}

func (ma *MessageApplication) HandleMessage(ctx context.Context, dto MessageAppeDTO) (*MessageAppeDTO, error) {
	conversationId := message_entity.GetConversationId(dto.SendId, dto.RecvId, dto.ConvType)

	err := ma.checkMember(ctx, dto.SendId, conversationId)

	if err != nil {
		return nil, err
	}

	messageId, err := snow.GenerateSnowId(int(ma.config.App.MachineID))
	if err != nil {
		return nil, ErrUnknown
	}

	// TODO: 后期可以优化为 lua 脚本
	seq, err := ma.messageCache.GetMessageLatestSeq(ctx, conversationId)

	if err != nil {
		if !errors.Is(err, message_entity.ErrConversationNotCreated) {
			return &MessageAppeDTO{
				ClientMsgId: dto.ClientMsgId,
				MessageId:   messageId,
				Status:      string(protocol.AckStatusFailed),
			}, err
		} else {
			curSeq, err := ma.conversationRepository.GetConvSeq(ctx, conversationId)
			if err != nil {
				return &MessageAppeDTO{
					ClientMsgId: dto.ClientMsgId,
					MessageId:   messageId,
					Status:      string(protocol.AckStatusFailed),
				}, nil
			}

			err = ma.messageCache.SetMessageSeq(ctx, conversationId, curSeq)
			if err != nil {
				// 设置缓存失败，缓存已经被创建，防止消息乱序，直接报错
				return &MessageAppeDTO{
					ClientMsgId: dto.ClientMsgId,
					MessageId:   messageId,
					Status:      string(protocol.AckStatusFailed),
				}, nil
			}
		}
	}

	seq, err = ma.messageCache.IncrMessageLatestSeq(ctx, conversationId)
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
			// 不影响主流程
			log.Printf("warn: update sender uc failed: %v", err)
		}

		if err := userConvRepo.UpdateSyncSeq(ctx, userConv); err != nil {
			// 不影响主流程
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
) ([]MessageAppeDTO, error) {
	uconvs, err := ma.userConversationRepository.ListByUser(ctx, userId)
	if err != nil {
		return nil, ErrConversationNotFound
	}

	convIds := make([]string, 0, len(uconvs))
	for _, uc := range uconvs {
		convIds = append(convIds, uc.ConversationId)
	}
	convs, err := ma.conversationRepository.ListByIds(ctx, convIds)
	syncMap := make(map[string]int64)

	for _, c := range convs {
		syncMap[c.ConversationId] = c.LatestSeq
	}

	msgs, _ := ma.messageRepository.ListLatestByConversations(
		ctx,
		convIds,
	)

	type syncJob struct {
		userConv *message_entity.UserConversation
		lastSeq  int64
	}
	jobs := make([]syncJob, 0, len(uconvs))

	for _, uconv := range uconvs {
		lastSeq := syncMap[uconv.ConversationId]
		if lastSeq < uconv.LatestSyncSeq {
			// 当前会话的最大值不需要再更新了
			continue
		}
		jobs = append(jobs, syncJob{
			userConv: uconv,
			lastSeq:  lastSeq,
		})
	}

	if len(jobs) > 0 {
		go func(jobs []syncJob) {
			for _, job := range jobs {
				job.userConv.SyncReadSeq(job.lastSeq)
				if err := ma.userConversationRepository.UpdateSyncSeq(
					context.Background(),
					job.userConv,
				); err != nil {
					log.Printf("marn: async update sync seq failed: %v", err)
				}
			}
		}(jobs)
	}

	return toMessagesAppDTO(msgs), nil
}
