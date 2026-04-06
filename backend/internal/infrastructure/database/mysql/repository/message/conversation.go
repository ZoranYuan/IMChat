package message_repository

import (
	"context"

	message_entity "IM_backend/internal/domain/message/entity"
	message_repository_interface "IM_backend/internal/domain/message/repository"
	"IM_backend/internal/infrastructure/database/mysql/model"

	"gorm.io/gorm"
)

type ConversationRepository struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) message_repository_interface.ConversationRepositoryInterface {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) WithTx(tx any) message_repository_interface.ConversationRepositoryInterface {
	return &ConversationRepository{
		db: tx.(*gorm.DB),
	}
}

// 创建会话
func (r *ConversationRepository) Save(ctx context.Context, conv *message_entity.Conversation) error {
	return r.db.WithContext(ctx).Create(toConversationModel(conv)).Error
}

// 根据 ID 查询
func (r *ConversationRepository) GetById(ctx context.Context, id string) (*message_entity.Conversation, error) {
	var m model.Conversation

	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", id).
		First(&m).Error

	if err != nil {
		return nil, err
	}

	return toConversationDomain(&m), nil
}

// 更新 LastSeq（发消息核心操作）
func (r *ConversationRepository) UpdateLastSeq(
	ctx context.Context,
	conversationId string,
	seq int64,
) error {

	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("conversation_id = ?", conversationId).
		Update("last_seq", seq).Error
}
