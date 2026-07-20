package conversation

import (
	"context"
	"errors"

	conversationrepo "IM_backend/internal/application/ports/persistence/repository/conversation"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

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
	uc *conversationentity.UserConversation,
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
	ucs []*conversationentity.UserConversation,
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
	uc *conversationentity.UserConversation,
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
	uc *conversationentity.UserConversation,
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
		return conversationentity.ErrDuplicateCreation
	}

	return nil
}

func (r *UserConversationRepository) WithTx(tx any) conversationrepo.UserConversationRepository {
	return &UserConversationRepository{
		db: tx.(*gorm.DB),
	}
}

func (r *UserConversationRepository) GetUsersByConversationID(ctx context.Context, conversationID string) ([]string, error) {
	var userIds []string

	err := r.db.
		WithContext(ctx).
		Model(&model.UserConversation{}).
		Where("conversation_id = ?", conversationID).
		Pluck("user_id", &userIds).Error

	return userIds, err
}

func (r *UserConversationRepository) GetUserConversationsByUserId(ctx context.Context, userId string) ([]*conversationentity.UserConversation, error) {
	if userId == "" {
		return nil, errors.New("用户标识不能为空")
	}

	var userConversations []*model.UserConversation

	err := r.db.
		WithContext(ctx).
		Model(&model.UserConversation{}).
		Where("user_id = ?", userId).
		Find(&userConversations).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
	}

	result := make([]*conversationentity.UserConversation, 0, len(userConversations))
	for _, userConversation := range userConversations {
		result = append(result, toUserConversationDomain(userConversation))
	}

	return result, nil
}

func (r *UserConversationRepository) GetUserConversation(
	ctx context.Context,
	userId string,
	conversationId string,
) (*conversationentity.UserConversation, error) {

	var m model.UserConversation

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ?", userId, conversationId).
		First(&m).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, conversationentity.ErrConversationNotCreated
		}
	}

	return toUserConversationDomain(&m), nil
}

func (r *UserConversationRepository) ListByUser(
	ctx context.Context,
	userId string,
) ([]*conversationentity.UserConversation, error) {

	var models []*model.UserConversation

	err := r.db.WithContext(ctx).
		Where("user_id = ?", userId).
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*conversationentity.UserConversation, 0, len(models))
	for _, m := range models {
		result = append(result, toUserConversationDomain(m))
	}

	return result, nil
}
