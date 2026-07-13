package message

import (
	"context"
	"errors"

	messageport "IM_backend/internal/application/ports/persistence/repository/message"
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

	"gorm.io/gorm"
)

type MessageVideoRepository struct{ db *gorm.DB }

func NewMessageVideoRepository(db *gorm.DB) *MessageVideoRepository {
	return &MessageVideoRepository{db: db}
}
func (r *MessageVideoRepository) WithTx(tx any) messageport.MessageVideoRepository {
	return &MessageVideoRepository{db: tx.(*gorm.DB)}
}
func toMessageVideoModel(e *messageentity.MessageVideo) *model.MessageVideo {
	if e == nil {
		return nil
	}
	return &model.MessageVideo{MessageId: e.MessageId, FileId: e.FileId, CoverFileId: e.CoverFileId, DurationMs: e.DurationMs, Width: e.Width, Height: e.Height, URL: e.URL}
}
func toMessageVideoDomain(m *model.MessageVideo) *messageentity.MessageVideo {
	if m == nil {
		return nil
	}
	return &messageentity.MessageVideo{MessageId: m.MessageId, FileId: m.FileId, CoverFileId: m.CoverFileId, DurationMs: m.DurationMs, Width: m.Width, Height: m.Height, URL: m.URL}
}
func (r *MessageVideoRepository) Create(ctx context.Context, item *messageentity.MessageVideo) error {
	return r.db.WithContext(ctx).Create(toMessageVideoModel(item)).Error
}
func (r *MessageVideoRepository) GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageVideo, error) {
	var m model.MessageVideo
	if err := r.db.WithContext(ctx).Where("message_id = ?", messageId).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toMessageVideoDomain(&m), nil
}

func (r *MessageVideoRepository) BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageVideo, error) {
	if len(messageIds) == 0 {
		return map[string]*messageentity.MessageVideo{}, nil
	}
	var models []model.MessageVideo
	if err := r.db.WithContext(ctx).Where("message_id IN (?)", messageIds).Find(&models).Error; err != nil {
		return nil, err
	}
	result := make(map[string]*messageentity.MessageVideo, len(models))
	for i := range models {
		result[models[i].MessageId] = toMessageVideoDomain(&models[i])
	}
	return result, nil
}
