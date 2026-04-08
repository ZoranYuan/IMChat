package message_repository_interface

import (
	message_entity "IM_backend/internal/domain/message/entity"
	"context"
)

type ConversationRepositoryInterface interface {

	// 创建会话
	Save(ctx context.Context, conv *message_entity.Conversation) error

	// 根据 ID 获取会话
	GetById(ctx context.Context, conversationId string) (*message_entity.Conversation, error)

	// 更新会话的最新 seq
	Upsert(
		ctx context.Context,
		domain *message_entity.Conversation,
	) error

	WithTx(tx any) ConversationRepositoryInterface
}
