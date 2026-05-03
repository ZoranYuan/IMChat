package message

import (
	messageentity "IM_backend/internal/domain/message/entity"
	"context"
)

type UserConversationRepository interface {

	// 创建用户会话关系
	CreateUserConversation(ctx context.Context, uc *messageentity.UserConversation) error

	GetUsersByConversationID(ctx context.Context, conversationID string) ([]string, error)

	BatchUpdateSyncSeq(
		ctx context.Context,
		ucs []*messageentity.UserConversation,
	) error

	// 获取单个用户会话
	GetUserConversation(
		ctx context.Context,
		userId string,
		conversationId string,
	) (*messageentity.UserConversation, error)

	UpdateSyncSeq(
		ctx context.Context,
		uc *messageentity.UserConversation,
	) error

	UpdateReadSeq(
		ctx context.Context,
		uc *messageentity.UserConversation,
	) error

	ListByUser(
		ctx context.Context,
		userId string,
	) ([]*messageentity.UserConversation, error)

	WithTx(tx any) UserConversationRepository
}
