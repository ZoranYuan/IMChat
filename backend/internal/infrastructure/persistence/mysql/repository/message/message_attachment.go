package message

import (
	messageport "IM_backend/internal/application/ports/persistence/repository/message"
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"context"
	"time"

	"gorm.io/gorm"
)

type MessageAttachmentRepository struct {
	db *gorm.DB
}

func NewMessageAttachmentRepository(db *gorm.DB) *MessageAttachmentRepository {
	return &MessageAttachmentRepository{
		db: db,
	}
}

func (m *MessageAttachmentRepository) WithTx(tx any) messageport.MessageAttachmentsRepository {
	return &MessageAttachmentRepository{db: tx.(*gorm.DB)}
}

func toMessageAttachmentModel(e *messageentity.MessageAttachment) *model.MessageAttachment {
	return &model.MessageAttachment{
		AttachmentId:   e.AttachmentId,
		MessageId:      e.MessageId,
		ConversationId: e.ConversationId,
		FileId:         e.FileId,
		Kind:           e.Kind,
		ExpireAt:       e.ExpireAt,
		CreatedAt:      e.CreatedAt,
	}
}

func toMessageAttachmentDomain(m *model.MessageAttachment) *messageentity.MessageAttachment {
	return &messageentity.MessageAttachment{
		AttachmentId:   m.AttachmentId,
		MessageId:      m.MessageId,
		ConversationId: m.ConversationId,
		FileId:         m.FileId,
		Kind:           m.Kind,
		ExpireAt:       m.ExpireAt,
		CreatedAt:      m.CreatedAt,
	}
}

func (m *MessageAttachmentRepository) Create(ctx context.Context, item *messageentity.MessageAttachment) error {
	return m.db.WithContext(ctx).Create(toMessageAttachmentModel(item)).Error
}

func (m *MessageAttachmentRepository) BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageAttachment, error) {
	if len(messageIds) == 0 {
		return map[string]*messageentity.MessageAttachment{}, nil
	}

	var rows []model.MessageAttachment
	if err := m.db.WithContext(ctx).Where("message_id IN (?)", messageIds).Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string]*messageentity.MessageAttachment, len(rows))
	for i := range rows {
		result[rows[i].MessageId] = toMessageAttachmentDomain(&rows[i])
	}
	return result, nil
}

func (m *MessageAttachmentRepository) CanUserAccessAttachment(ctx context.Context, userId string, attachmentId string) (bool, error) {
	var exists bool
	err := m.db.WithContext(ctx).Raw(`
			SELECT EXISTS (
					SELECT 1
					FROM message_attachments AS ma
					JOIN user_conversations AS uc
						ON uc.conversation_id = ma.conversation_id
						AND uc.user_id = ?
					WHERE ma.attachment_id = ?
						AND ma.expire_at > ?
					LIMIT 1
			)
        `, userId, attachmentId, time.Now().UnixMilli()).Scan(&exists).Error
	return exists, err
}

func (m *MessageAttachmentRepository) FindUserAccessAttachment(ctx context.Context, userId string, attachmentId string) (*messageentity.MessageAttachment, error) {
	var row model.MessageAttachment
	err := m.db.WithContext(ctx).
		Table("message_attachments AS ma").
		Select("ma.*").
		Joins(`JOIN user_conversations AS uc
			ON uc.conversation_id = ma.conversation_id
			AND uc.user_id = ?`, userId).
		Where("ma.attachment_id = ? AND ma.expire_at > ?", attachmentId, time.Now().UnixMilli()).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.AttachmentId == "" {
		return nil, nil
	}
	return toMessageAttachmentDomain(&row), nil
}
