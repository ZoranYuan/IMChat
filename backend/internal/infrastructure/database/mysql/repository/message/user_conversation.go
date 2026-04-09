package message_repository

import (
	"context"

	message_entity "IM_backend/internal/domain/message/entity"
	message_repository_interface "IM_backend/internal/domain/message/repository"
	"IM_backend/internal/infrastructure/database/mysql/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserConversationRepository struct {
	db *gorm.DB
}

func NewUserConversationRepository(db *gorm.DB) *UserConversationRepository {
	return &UserConversationRepository{db: db}
}

// 创建用户会话关系
func (r *UserConversationRepository) Save(ctx context.Context, uc *message_entity.UserConversation) error {
	return r.db.WithContext(ctx).Create(toUserConversationModel(uc)).Error
}

// 更新已读 seq
func (r *UserConversationRepository) Upsert(
	ctx context.Context,
	domain *message_entity.UserConversation,
) error {

	m := toUserConversationModel(domain)

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "conversation_id"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"latest_read_seq": m.LatestReadSeq,
			}),
		}).
		Create(m).Error
}

func (r *UserConversationRepository) WithTx(tx any) message_repository_interface.UserConversationRepositoryInterface {
	return &UserConversationRepository{
		db: tx.(*gorm.DB),
	}
}

// 获取用户会话
func (r *UserConversationRepository) Get(
	ctx context.Context,
	userId string,
	conversationId string,
) (*message_entity.UserConversation, error) {

	var m model.UserConversation

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ?", userId, conversationId).
		First(&m).Error

	if err != nil {
		return nil, err
	}

	return toUserConversationDomain(&m), nil
}

// 获取用户所有会话（用于会话列表）
func (r *UserConversationRepository) ListByUser(
	ctx context.Context,
	userId string,
) ([]*message_entity.UserConversation, error) {

	var models []*model.UserConversation

	err := r.db.WithContext(ctx).
		Where("user_id = ?", userId).
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*message_entity.UserConversation, 0, len(models))
	for _, m := range models {
		result = append(result, toUserConversationDomain(m))
	}

	return result, nil
}
