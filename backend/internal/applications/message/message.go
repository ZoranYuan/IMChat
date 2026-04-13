package application_message

import (
	"IM_backend/configs"
	message_cache_interface "IM_backend/internal/applications/interface/cache/message"
	mq_interface "IM_backend/internal/applications/interface/mq"
	tx_repository_interface "IM_backend/internal/applications/interface/repository/tx_manager"
	message_entity "IM_backend/internal/domain/message/entity"
	message_repository_interface "IM_backend/internal/domain/message/repository"
	message_valueobject "IM_backend/internal/domain/message/value_object"
	"IM_backend/internal/infrastructure/pkg/snow"
	"IM_backend/internal/protocol"
	"context"
	"log"
	"math"
	"sort"

	"gorm.io/gorm"
)

type MessageApplication struct {
	config                     configs.Config
	messageCache               message_cache_interface.MessageCacheInterface
	messageRepository          message_repository_interface.MessageRepositoryInterface
	txManager                  tx_repository_interface.TxRepositoryInterface
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface
	conversationRepository     message_repository_interface.ConversationRepositoryInterface
	taskManager                mq_interface.TaskManager
}

func NewMessageApplication(
	config configs.Config,
	messageCache message_cache_interface.MessageCacheInterface,
	txManager tx_repository_interface.TxRepositoryInterface,
	taskManager mq_interface.TaskManager,
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface,
	conversationRepository message_repository_interface.ConversationRepositoryInterface,
	messageRepository message_repository_interface.MessageRepositoryInterface,
) *MessageApplication {
	return &MessageApplication{
		config:                     config,
		messageCache:               messageCache,
		txManager:                  txManager,
		taskManager:                taskManager,
		messageRepository:          messageRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
	}
}

func (wa *MessageApplication) HandleMessage(ctx context.Context, dto MessageAppeDTO) (*protocol.AckEvent, error) {
	conversationId := message_entity.GetConversation(dto.SendId, dto.RecvId, dto.ConvType)
	messageId, err := snow.GenerateSnowId(int(wa.config.App.MachineID))
	if err != nil {
		return nil, ErrUnknown
	}

	// TODO: 这里如果没有，需要及时去查询数据库，而不是简单的初始化
	seq, err := wa.messageCache.GetConvLatestSeq(ctx, conversationId)
	if err != nil {
		return &protocol.AckEvent{
			ClientMsgId: dto.ClientMsgId,
			Status:      protocol.AckStatusFailed,
			ErrorMsg:    ErrConversationNotFound.Error(),
		}, nil
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

	err = wa.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		msgRepo := wa.messageRepository.WithTx(tx)
		convRepo := wa.conversationRepository.WithTx(tx)
		userConvRepo := wa.userConversationRepository.WithTx(tx)

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
		)

		userConv := message_entity.BuildUserConversation(
			dto.SendId,
			conversationId,
			seq,
			seq,
		)

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
		return &protocol.AckEvent{
			ClientMsgId: dto.ClientMsgId,
			Status:      protocol.AckStatusFailed,
			ErrorMsg:    ErrConversationNotFound.Error(),
		}, nil
	}

	_, err = wa.messageCache.IncrConvSeq(ctx, conversationId)
	if err != nil {
		// TODO 异步补偿机制
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
		ConvType:       dto.ConvType,
		CType:          dto.CType,
		Content:        dto.Content,
		SendTime:       message.SendTime,
	}

	go func() {
		if err := wa.taskManager.SendMessage(
			context.Background(),
			"chat",
			conversationId,
			messageEvent,
		); err != nil {
			log.Printf("mq send failed: %v", err)
			// TODO: 后续做补偿（Outbox）
		}
	}()

	return &protocol.AckEvent{
		ClientMsgId: dto.ClientMsgId,
		MessageId:   messageId,
		Status:      protocol.AckStatusSent,
	}, nil
}

func (wa *MessageApplication) GetHistoryMessages(
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
		uc, err := wa.userConversationRepository.Get(ctx, userId, conversationId)
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

	msgs, err := wa.messageRepository.GetHistoryMessage(
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

func (wa *MessageApplication) GetOfflineMessages(
	ctx context.Context,
	userId string,
) ([]MessageAppeDTO, error) {
	uconvs, err := wa.userConversationRepository.ListByUser(ctx, userId)
	if err != nil {
		return nil, ErrConversationNotFound
	}

	convIds := make([]string, 0, len(uconvs))
	for _, uc := range uconvs {
		convIds = append(convIds, uc.ConversationId)
	}
	convs, err := wa.conversationRepository.ListByIds(ctx, convIds)
	syncMap := make(map[string]int64)

	for _, c := range convs {
		syncMap[c.ConversationId] = c.LastSeq
	}

	msgs, _ := wa.messageRepository.ListLatestByConversations(
		ctx,
		convIds,
	)

	// TODO: 后期改为异步修改数据库状态
	for _, uconv := range uconvs {
		lastSeq := syncMap[uconv.ConversationId]
		if lastSeq < uconv.LatestSyncSeq {
			// 当前会话的最大值不需要再更新了
			continue
		}
		uconv.SyncReadSeq(lastSeq)
		wa.userConversationRepository.UpdateSyncSeq(
			ctx,
			uconv,
		)
	}

	return toMessagesAppDTO(msgs), nil
}
