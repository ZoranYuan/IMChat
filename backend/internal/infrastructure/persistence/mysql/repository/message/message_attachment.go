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

func (m *MessageAttachmentRepository) BatchGetByMessageIDs(ctx context.Context, messageIds []string, isActivate bool) (map[string]*messageentity.MessageAttachment, error) {
	if len(messageIds) == 0 {
		return map[string]*messageentity.MessageAttachment{}, nil
	}

	query := m.db.WithContext(ctx).Where("message_id IN (?)", messageIds)
	if isActivate {
		query = query.Where("expire_at > ?", time.Now().UnixMilli())
	}

	var rows []model.MessageAttachment
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string]*messageentity.MessageAttachment, len(rows))
	for i := range rows {
		result[rows[i].MessageId] = toMessageAttachmentDomain(&rows[i])
	}
	return result, nil
}

func (m *MessageAttachmentRepository) FindUserAccessAttachments(ctx context.Context, userId string, attachmentIds []string) (map[string]*messageentity.MessageAttachment, error) {
	result := make(map[string]*messageentity.MessageAttachment, len(attachmentIds))
	if userId == "" || len(attachmentIds) == 0 {
		return result, nil
	}

	var rows []model.MessageAttachment
	err := m.db.WithContext(ctx).
		Table("message_attachments AS ma").
		Select("ma.*").
		Joins("JOIN user_conversations AS uc ON uc.conversation_id = ma.conversation_id").
		Where("uc.user_id = ? AND ma.attachment_id IN (?) AND ma.expire_at > ?", userId, attachmentIds, time.Now().UnixMilli()).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	for i := range rows {
		attachment := toMessageAttachmentDomain(&rows[i])
		result[attachment.AttachmentId] = attachment
	}
	return result, nil
}

func (m *MessageAttachmentRepository) CanUserAccessAttachment(ctx context.Context, userId string, attachmentId string) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).
		Table("message_attachments AS ma").
		Joins("JOIN user_conversations AS uc ON uc.conversation_id = ma.conversation_id").
		Where("uc.user_id = ? AND ma.attachment_id = ? AND ma.expire_at > ?", userId, attachmentId, time.Now().UnixMilli()).
		Count(&count).Error
	return count > 0, err
}

func (m *MessageAttachmentRepository) FindUserAccessAttachment(ctx context.Context, userId string, attachmentId string) (*messageentity.MessageAttachment, error) {
	var row model.MessageAttachment
	err := m.db.WithContext(ctx).
		Table("message_attachments AS ma").
		Select("ma.*").
		Joins("JOIN user_conversations AS uc ON uc.conversation_id = ma.conversation_id").
		Where("uc.user_id = ? AND ma.attachment_id = ? AND ma.expire_at > ?", userId, attachmentId, time.Now().UnixMilli()).
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
