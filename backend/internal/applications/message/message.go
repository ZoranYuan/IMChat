package application_message

import (
	"IM_backend/configs"
	message_cache_interface "IM_backend/internal/applications/interface/cache/message"
	tx_repository_interface "IM_backend/internal/applications/interface/repository/tx_manager"
	message_entity "IM_backend/internal/domain/message/entity"
	message_repository_interface "IM_backend/internal/domain/message/repository"
	message_valueobject "IM_backend/internal/domain/message/value_object"
	"IM_backend/internal/infrastructure/pkg/snow"
	"context"
	"errors"
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
}

func NewWatchApplication(
	config configs.Config,
	messageCache message_cache_interface.MessageCacheInterface,
	txManager tx_repository_interface.TxRepositoryInterface,
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface,
	conversationRepository message_repository_interface.ConversationRepositoryInterface,
	messageRepository message_repository_interface.MessageRepositoryInterface,
) *MessageApplication {
	return &MessageApplication{
		config:                     config,
		messageCache:               messageCache,
		txManager:                  txManager,
		messageRepository:          messageRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
	}
}

// 处理消息推送
func (w *MessageApplication) HandleMessage(ctx context.Context, dto MessageAppeDTO) error {
	conversationId := max(dto.UserId, dto.RecvId) + min(dto.UserId, dto.RecvId)
	messageId, err := snow.GenerateSnowId(int(w.config.App.MachineID))
	if err != nil {
		return err
	}

	seq, err := w.messageCache.GetConvLatestSeq(ctx, conversationId)
	if err != nil {
		return err
	}

	message := message_entity.NewMessage(
		messageId,
		conversationId,
		dto.UserId,
		seq,
		dto.Content,
		dto.VideoTime,
		message_valueobject.CType(dto.Ctype),
		message_valueobject.Normal,
	)

	// 发消息数据入库
	if err := w.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		msgRepo := w.messageRepository.WithTx(tx)
		convRepo := w.conversationRepository.WithTx(tx)
		userConvRepo := w.userConversationRepository.WithTx(tx)

		_, err := convRepo.GetById(ctx, conversationId)
		if err != nil {
			// 这里只作为一个弱一致性兜底机制，会话在第一次加入房间或者第一次私聊就已经创建好了
			if errors.Is(err, ErrConversationNotFound) {
				conversation := message_entity.NewConversation(
					conversationId,
					dto.UserId,
					dto.RecvId,
					dto.Convtype,
				)

				if err := convRepo.Save(ctx, conversation); err != nil {
					return ErrConversationCreate
				}
			} else {
				log.Println("failed to create conv, ", err)
				return ErrUnknown
			}
		}

		if err := msgRepo.Save(ctx, message); err != nil {
			return ErrMessageSave
		}

		if err := convRepo.UpdateLastSeq(ctx, conversationId, seq); err != nil {
			return ErrConversationUpdateSeq
		}

		// 降级
		if err := userConvRepo.UpdateReadSeq(ctx, dto.UserId, conversationId, seq); err != nil {
			log.Printf("warn: update read seq failed: %v", err)

			// TODO 异步补偿
		}

		return nil
	}); err != nil {
		return err
	}

	// TODO 使用 producer 推送给 mq 层
	return nil
}
