package message

import (
	"context"
	"errors"

	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

	messageport "IM_backend/internal/application/ports/persistence/repository/message"

	"gorm.io/gorm"
)

type MessageImageRepository struct {
	db *gorm.DB
}

func NewMessageImageRepository(db *gorm.DB) *MessageImageRepository {
	return &MessageImageRepository{db: db}
}

func (r *MessageImageRepository) WithTx(tx any) messageport.MessageImageRepository {
	return &MessageImageRepository{db: tx.(*gorm.DB)}
}

func toMessageImageModel(e *messageentity.MessageImage) *model.MessageImage {
	if e == nil {
		return nil
	}
	return &model.MessageImage{MessageId: e.MessageId, FileId: e.FileId, ThumbFileId: e.ThumbFileId, Width: e.Width, Height: e.Height, MimeType: e.MimeType, Size: e.Size, URL: e.URL}
}

func toMessageImageDomain(m *model.MessageImage) *messageentity.MessageImage {
	if m == nil {
		return nil
	}
	return &messageentity.MessageImage{MessageId: m.MessageId, FileId: m.FileId, ThumbFileId: m.ThumbFileId, Width: m.Width, Height: m.Height, MimeType: m.MimeType, Size: m.Size, URL: m.URL}
}

func (r *MessageImageRepository) Create(ctx context.Context, item *messageentity.MessageImage) error {
	return r.db.WithContext(ctx).Create(toMessageImageModel(item)).Error
}

func (r *MessageImageRepository) GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageImage, error) {
	var m model.MessageImage
	if err := r.db.WithContext(ctx).Where("message_id = ?", messageId).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toMessageImageDomain(&m), nil
}

func (r *MessageImageRepository) BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageImage, error) {
	if len(messageIds) == 0 {
		return map[string]*messageentity.MessageImage{}, nil
	}
	var models []model.MessageImage
	if err := r.db.WithContext(ctx).Where("message_id IN (?)", messageIds).Find(&models).Error; err != nil {
		return nil, err
	}
	result := make(map[string]*messageentity.MessageImage, len(models))
	for i := range models {
		result[models[i].MessageId] = toMessageImageDomain(&models[i])
	}
	return result, nil
}
