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

func (w *MessageApplication) HandleMessage(ctx context.Context, dto MessageAppeDTO) error {
	conversationId := message_entity.GetConversation(dto.SendId, dto.RecvId, dto.ConvType)

	seq, err := w.messageCache.GetConvLatestSeq(ctx, conversationId)
	if err != nil {
		return ErrConversationNotFound
	}

	messageId, err := snow.GenerateSnowId(int(w.config.App.MachineID))
	if err != nil {
		return ErrUnknown
	}

	message := message_entity.BuildMessage(
		messageId,
		conversationId,
		dto.SendId,
		seq,
		dto.Content,
		dto.VideoTime,
		message_valueobject.CType(dto.Ctype),
	)

	err = w.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		msgRepo := w.messageRepository.WithTx(tx)
		convRepo := w.conversationRepository.WithTx(tx)
		userConvRepo := w.userConversationRepository.WithTx(tx)

		message := message_entity.BuildMessage(
			messageId,
			conversationId,
			dto.SendId,
			seq,
			dto.Content,
			dto.VideoTime,
			message_valueobject.CType(dto.Ctype),
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
		)

		if err := msgRepo.Save(ctx, message); err != nil {
			// TODO 异步补偿机制
			return ErrMessageSave
		}

		if err := convRepo.Upsert(ctx, conv); err != nil {
			return ErrConversationUpdateSeq
		}

		if err := userConvRepo.Upsert(ctx, userConv); err != nil {
			// 不影响主流程
			log.Printf("warn: update sender uc failed: %v", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	_, err = w.messageCache.IncrConvSeq(ctx, conversationId)
	if err != nil {
		// TODO 异步补偿机制
	}

	value := ctx.Value("op")

	op, ok := value.(string)

	if !ok || op == "" {
		return ErrUnknown
	}

	event := protocol.MessageEvent{
		MessageId:      messageId,
		ConversationId: conversationId,
		SendId:         dto.SendId,
		RecvId:         dto.RecvId,
		ConvType:       dto.ConvType,
		CType:          dto.Ctype,
		Content:        dto.Content,
		SendTime:       message.SendTime,
	}

	// 7. 发送 MQ
	if err := w.taskManager.SendMessage(
		ctx,
		"chat",
		conversationId,
		event,
	); err != nil {
		log.Printf("mq send failed: %v", err)
		return ErrUnknown
	}

	return nil
}
