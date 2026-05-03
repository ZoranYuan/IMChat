package message

import (
	"context"
	"errors"

	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ConversationRepository struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) WithTx(tx any) messagerepo.ConversationRepository {
	return &ConversationRepository{
		db: tx.(*gorm.DB),
	}
}

// 创建会话
func (r *ConversationRepository) CreateConversation(ctx context.Context, conv *messageentity.Conversation) error {
	m := toConversationModel(conv)
	return r.db.WithContext(ctx).Create(&m).Error
}

// 根据 ID 查询
func (r *ConversationRepository) GetByID(ctx context.Context, id string) (*messageentity.Conversation, error) {
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

func (r *ConversationRepository) Upsert(
	ctx context.Context,
	domain *messageentity.Conversation,
) error {
	m := toConversationModel(domain)

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "conversation_id"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"latest_message_id": m.LatestMessageId,
				"latest_seq":        gorm.Expr("GREATEST(latest_seq, ?)", m.LatestSeq),
			}),
		}).
		Create(m).Error

}

func (r *ConversationRepository) GetConversationSeq(ctx context.Context, conversationID string) (int64, error) {
	var seq int64

	err := r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("conversation_id = ?", conversationID).
		Pluck("latest_seq", &seq).Error

	if err != nil {
		return 0, err
	}

	return seq, nil
}

func (r *ConversationRepository) ListByIDs(
	ctx context.Context,
	ids []string,
) ([]*messageentity.Conversation, error) {

	if len(ids) == 0 {
		return []*messageentity.Conversation{}, nil
	}

	var models []model.Conversation

	err := r.db.WithContext(ctx).
		Where("conversation_id IN ?", ids).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	// 转换为 domain
	result := make([]*messageentity.Conversation, 0, len(models))
	for _, m := range models {
		result = append(result, toConversationDomain(&m))
	}

	return result, nil
}
