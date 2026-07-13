package message

import (
	"context"
	"errors"

	messageport "IM_backend/internal/application/ports/persistence/repository/message"
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

	"gorm.io/gorm"
)

type MessageStickerRepository struct{ db *gorm.DB }

func NewMessageStickerRepository(db *gorm.DB) *MessageStickerRepository {
	return &MessageStickerRepository{db: db}
}
func (r *MessageStickerRepository) WithTx(tx any) messageport.MessageStickerRepository {
	return &MessageStickerRepository{db: tx.(*gorm.DB)}
}
func toMessageStickerModel(e *messageentity.MessageSticker) *model.MessageSticker {
	if e == nil {
		return nil
	}
	return &model.MessageSticker{MessageId: e.MessageId, StickerId: e.StickerId, PackId: e.PackId, URL: e.URL, Width: e.Width, Height: e.Height}
}
func toMessageStickerDomain(m *model.MessageSticker) *messageentity.MessageSticker {
	if m == nil {
		return nil
	}
	return &messageentity.MessageSticker{MessageId: m.MessageId, StickerId: m.StickerId, PackId: m.PackId, URL: m.URL, Width: m.Width, Height: m.Height}
}
func (r *MessageStickerRepository) Create(ctx context.Context, item *messageentity.MessageSticker) error {
	return r.db.WithContext(ctx).Create(toMessageStickerModel(item)).Error
}
func (r *MessageStickerRepository) GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageSticker, error) {
	var m model.MessageSticker
	if err := r.db.WithContext(ctx).Where("message_id = ?", messageId).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toMessageStickerDomain(&m), nil
}

func (r *MessageStickerRepository) BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageSticker, error) {
	if len(messageIds) == 0 {
		return map[string]*messageentity.MessageSticker{}, nil
	}
	var models []model.MessageSticker
	if err := r.db.WithContext(ctx).Where("message_id IN (?)", messageIds).Find(&models).Error; err != nil {
		return nil, err
	}
	result := make(map[string]*messageentity.MessageSticker, len(models))
	for i := range models {
		result[models[i].MessageId] = toMessageStickerDomain(&models[i])
	}
	return result, nil
}
