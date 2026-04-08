package message_repository_interface

import (
	message_entity "IM_backend/internal/domain/message/entity"
	"context"
)

type UserConversationRepositoryInterface interface {

	// 创建用户会话关系
	Save(ctx context.Context, uc *message_entity.UserConversation) error

	// 获取单个用户会话
	Get(
		ctx context.Context,
		userId string,
		conversationId string,
	) (*message_entity.UserConversation, error)

	// 更新已读位置
	Upsert(
		ctx context.Context,
		domain *message_entity.UserConversation,
	) error

	// 获取用户所有会话（会话列表）
	ListByUser(
		ctx context.Context,
		userId string,
	) ([]*message_entity.UserConversation, error)

	WithTx(tx any) UserConversationRepositoryInterface
}
