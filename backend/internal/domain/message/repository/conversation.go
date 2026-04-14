package message_repository_interface

import (
	message_entity "IM_backend/internal/domain/message/entity"
	"context"
)

type ConversationRepositoryInterface interface {
	CreateConversation(ctx context.Context, conv *message_entity.Conversation) error

	GetConvSeq(ctx context.Context, convId string) (int64, error)

	GetById(ctx context.Context, conversationId string) (*message_entity.Conversation, error)

	Upsert(
		ctx context.Context,
		domain *message_entity.Conversation,
	) error

	ListByIds(
		ctx context.Context,
		ids []string,
	) ([]*message_entity.Conversation, error)

	WithTx(tx any) ConversationRepositoryInterface
}
