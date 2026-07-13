package message

import (
	"context"
	"errors"

	messageport "IM_backend/internal/application/ports/persistence/repository/message"
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

	"gorm.io/gorm"
)

type MessageFileRepository struct {
	db *gorm.DB
}

func NewMessageFileRepository(db *gorm.DB) *MessageFileRepository {
	return &MessageFileRepository{db: db}
}

func (r *MessageFileRepository) WithTx(tx any) messageport.MessageFileRepository {
	return &MessageFileRepository{db: tx.(*gorm.DB)}
}

func toMessageFileModel(e *messageentity.MessageFile) *model.MessageFile {
	if e == nil {
		return nil
	}
	return &model.MessageFile{MessageId: e.MessageId, FileId: e.FileId, FileName: e.FileName, DownloadName: e.DownloadName, MimeType: e.MimeType, Size: e.Size, URL: e.URL}
}
func toMessageFileDomain(m *model.MessageFile) *messageentity.MessageFile {
	if m == nil {
		return nil
	}
	return &messageentity.MessageFile{MessageId: m.MessageId, FileId: m.FileId, FileName: m.FileName, DownloadName: m.DownloadName, MimeType: m.MimeType, Size: m.Size, URL: m.URL}
}

func (r *MessageFileRepository) Create(ctx context.Context, item *messageentity.MessageFile) error {
	return r.db.WithContext(ctx).Create(toMessageFileModel(item)).Error
}
func (r *MessageFileRepository) GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageFile, error) {
	var m model.MessageFile
	if err := r.db.WithContext(ctx).Where("message_id = ?", messageId).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toMessageFileDomain(&m), nil
}

func (r *MessageFileRepository) BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageFile, error) {
	if len(messageIds) == 0 {
		return map[string]*messageentity.MessageFile{}, nil
	}
	var models []model.MessageFile
	if err := r.db.WithContext(ctx).Where("message_id IN (?)", messageIds).Find(&models).Error; err != nil {
		return nil, err
	}
	result := make(map[string]*messageentity.MessageFile, len(models))
	for i := range models {
		result[models[i].MessageId] = toMessageFileDomain(&models[i])
	}
	return result, nil
}
