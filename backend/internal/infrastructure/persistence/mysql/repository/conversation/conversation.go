package conversation

import (
	"context"
	"errors"

	conversationrepo "IM_backend/internal/application/ports/persistence/repository/conversation"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

	"gorm.io/gorm"
)

type ConversationRepository struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) WithTx(tx any) conversationrepo.ConversationRepository {
	return &ConversationRepository{
		db: tx.(*gorm.DB),
	}
}

// 创建会话
func (r *ConversationRepository) CreateConversation(ctx context.Context, conv *conversationentity.Conversation) error {
	m := toConversationModel(conv)
	return r.db.WithContext(ctx).Create(&m).Error
}

// 根据 ID 查询
func (r *ConversationRepository) GetByID(ctx context.Context, id string) (*conversationentity.Conversation, error) {
	var m model.Conversation

	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", id).
		First(&m).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toConversationDomain(&m), nil
}

func (r *ConversationRepository) UpdateLatestSequence(
	ctx context.Context,
	domain *conversationentity.Conversation,
	updateLatestMessage bool,
) error {
	m := toConversationModel(domain)
	updates := map[string]interface{}{
		"latest_seq": gorm.Expr("GREATEST(latest_seq, ?)", m.LatestSeq),
	}
	if updateLatestMessage {
		updates["latest_message_id"] = gorm.Expr(
			"CASE WHEN latest_seq < ? THEN ? ELSE latest_message_id END",
			m.LatestSeq,
			m.LatestMessageId,
		)
	}

	result := r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("conversation_id = ?", m.ConversationId).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("conversation_id = ?", m.ConversationId).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return conversationentity.ErrConversationNotCreated
	}
	return nil
}

func (r *ConversationRepository) GetConversationSeq(
	ctx context.Context,
	conversationID string,
) (int64, error) {
	var conv model.Conversation

	err := r.db.WithContext(ctx).
		Select("latest_seq").
		Where("conversation_id = ?", conversationID).
		Take(&conv).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, conversationentity.ErrConversationNotCreated
	}
	if err != nil {
		return 0, err
	}

	return conv.LatestSeq, nil
}

func (r *ConversationRepository) ListByIDs(
	ctx context.Context,
	ids []string,
) ([]*conversationentity.Conversation, error) {

	if len(ids) == 0 {
		return []*conversationentity.Conversation{}, nil
	}

	var models []model.Conversation

	err := r.db.WithContext(ctx).
		Where("conversation_id IN ?", ids).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	// 转换为 domain
	result := make([]*conversationentity.Conversation, 0, len(models))
	for _, m := range models {
		result = append(result, toConversationDomain(&m))
	}

	return result, nil
}
