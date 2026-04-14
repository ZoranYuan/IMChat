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

func (r *UserConversationRepository) UpdateReadSeq(
	ctx context.Context,
	uc *message_entity.UserConversation,
) error {

	return r.db.WithContext(ctx).
		Model(&message_entity.UserConversation{}).
		Where("user_id = ? AND conversation_id = ?", uc.UserId, uc.ConversationId).
		Updates(map[string]interface{}{
			"last_read_seq": gorm.Expr(
				"GREATEST(last_read_seq, ?)",
				uc.LastReadSeq,
			),
		}).Error
}

func (r *UserConversationRepository) UpdateSyncSeq(
	ctx context.Context,
	uc *message_entity.UserConversation,
) error {

	return r.db.WithContext(ctx).
		Model(&message_entity.UserConversation{}).
		Where("user_id = ? AND conversation_id = ?", uc.UserId, uc.ConversationId).
		Updates(map[string]interface{}{
			"latest_sync_seq": gorm.Expr(
				"GREATEST(latest_sync_seq, ?)",
				uc.LatestSyncSeq,
			),
		}).Error
}

func (r *UserConversationRepository) CreateUserConversation(
	ctx context.Context,
	uc *message_entity.UserConversation,
) error {
	m := toUserConversationModel(uc)

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "conversation_id"},
			},
			DoNothing: true,
		}).
		Create(&model.UserConversation{
			UserId:         m.UserId,
			ConversationId: m.ConversationId,
		}).Error
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
