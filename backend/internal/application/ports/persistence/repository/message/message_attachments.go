package message

import (
	messageentity "IM_backend/internal/domain/message/entity"
	"context"
)

type MessageAttachmentsRepository interface {
	Create(ctx context.Context, item *messageentity.MessageAttachment) error
	BatchGetByMessageIDs(ctx context.Context, messageIds []string, isActivate bool) (map[string]*messageentity.MessageAttachment, error)
	FindUserAccessAttachments(ctx context.Context, userId string, attachmentIds []string) (map[string]*messageentity.MessageAttachment, error)
	WithTx(tx any) MessageAttachmentsRepository
	CanUserAccessAttachment(ctx context.Context, userId string, attachment_id string) (bool, error)
	FindUserAccessAttachment(ctx context.Context, userId string, attachment_id string) (*messageentity.MessageAttachment, error)
}
