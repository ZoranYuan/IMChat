package message_repository

import (
	"context"

	message_entity "IM_backend/internal/domain/message/entity"
	message_repository_interface "IM_backend/internal/domain/message/repository"
	"IM_backend/internal/infrastructure/database/mysql/model"

	"gorm.io/gorm"
)

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) WithTx(tx any) message_repository_interface.MessageRepositoryInterface {
	return &MessageRepository{
		db: tx.(*gorm.DB),
	}
}

// 保存消息
func (r *MessageRepository) Save(ctx context.Context, msg *message_entity.Message) error {
	m := toMessageModel(msg)
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *MessageRepository) GetHistoryMessage(
	ctx context.Context,
	conversationId string,
	maxSeq int64,
	limit int,
) ([]*message_entity.Message, error) {

	var models []*model.Message

	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND seq < ?", conversationId, maxSeq).
		Order("seq DESC").
		Limit(limit).
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*message_entity.Message, 0, len(models))
	for _, m := range models {
		result = append(result, toMessageDomain(m))
	}

	return result, nil
}

func (r *MessageRepository) ListLatestByConversations(
	ctx context.Context,
	convIDs []string,
) ([]*message_entity.Message, error) {

	var msgs []*model.Message

	err := r.db.WithContext(ctx).
		Raw(`
            SELECT *
			FROM messages
			WHERE conversation_id IN ?
			ORDER BY seq ASC
			LIMIT 1;
        `, convIDs).
		Scan(&msgs).Error

	var domains []*message_entity.Message

	for _, m := range msgs {
		domains = append(domains, toMessageDomain(m))
	}

	return domains, err
}
