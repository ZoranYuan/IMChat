package message_repository_interface

import (
	message_entity "IM_backend/internal/domain/message/entity"
	"context"
)

type UserConversationRepositoryInterface interface {

	// 创建用户会话关系
	CreateUserConversation(ctx context.Context, uc *message_entity.UserConversation) error

	GetUsersByConvId(ctx context.Context, convId string) ([]string, error)

	BatchUpdateSyncSeq(
		ctx context.Context,
		ucs []*message_entity.UserConversation,
	) error

	// 获取单个用户会话
	GetUserConversation(
		ctx context.Context,
		userId string,
		conversationId string,
	) (*message_entity.UserConversation, error)

	UpdateSyncSeq(
		ctx context.Context,
		uc *message_entity.UserConversation,
	) error

	UpdateReadSeq(
		ctx context.Context,
		uc *message_entity.UserConversation,
	) error

	ListByUser(
		ctx context.Context,
		userId string,
	) ([]*message_entity.UserConversation, error)

	WithTx(tx any) UserConversationRepositoryInterface
}
