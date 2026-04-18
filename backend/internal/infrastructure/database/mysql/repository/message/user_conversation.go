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

func (r *UserConversationRepository) upsert(
	ctx context.Context,
	m *model.UserConversation,
	assignments map[string]interface{},
) error {
	return r.db.WithContext(ctx).
		Model(&model.UserConversation{}).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "conversation_id"},
			},
			DoUpdates: clause.Assignments(assignments),
		}).
		Create(m).Error
}

func (r *UserConversationRepository) UpdateReadSeq(
	ctx context.Context,
	uc *message_entity.UserConversation,
) error {
	m := toUserConversationModel(uc)

	return r.upsert(ctx, m, map[string]interface{}{
		"last_read_seq": gorm.Expr(
			"GREATEST(last_read_seq, VALUES(last_read_seq))",
		),
	})
}

func (r *UserConversationRepository) BatchUpdateSyncSeq(
	ctx context.Context,
	ucs []*message_entity.UserConversation,
) error {

	if len(ucs) == 0 {
		return nil
	}

	ms := make([]*model.UserConversation, 0, len(ucs))
	for _, uc := range ucs {
		ms = append(ms, toUserConversationModel(uc))
	}

	return r.db.WithContext(ctx).
		Model(&model.UserConversation{}).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "conversation_id"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"latest_sync_seq": gorm.Expr(
					"GREATEST(latest_sync_seq, VALUES(latest_sync_seq))",
				),
			}),
		}).
		Create(&ms).Error
}

func (r *UserConversationRepository) UpdateSyncSeq(
	ctx context.Context,
	uc *message_entity.UserConversation,
) error {

	m := toUserConversationModel(uc)

	return r.upsert(ctx, m, map[string]interface{}{
		"latest_sync_seq": gorm.Expr(
			"GREATEST(latest_sync_seq, VALUES(latest_sync_seq))",
		),
	})
}

func (r *UserConversationRepository) CreateUserConversation(
	ctx context.Context,
	uc *message_entity.UserConversation,
) error {
	m := toUserConversationModel(uc)

	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "conversation_id"}},
			DoNothing: true,
		}).
		Create(&m)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return message_entity.ErrDuplicateCreate
	}

	return nil
}

func (r *UserConversationRepository) WithTx(tx any) message_repository_interface.UserConversationRepositoryInterface {
	return &UserConversationRepository{
		db: tx.(*gorm.DB),
	}
}

func (r *UserConversationRepository) GetUsersByConvId(ctx context.Context, convId string) ([]string, error) {
	var userIds []string

	err := r.db.
		WithContext(ctx).
		Model(&model.UserConversation{}).
		Where("conversation_id = ?", convId).
		Pluck("user_id", &userIds).Error

	return userIds, err
}

func (r *UserConversationRepository) GetUserConversation(
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
