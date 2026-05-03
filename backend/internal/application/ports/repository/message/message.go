package message_repository_interface

import (
	message_entity "IM_backend/internal/domain/message/entity"
	"context"
)

type MessageRepository interface {

	// 保存消息
	Save(ctx context.Context, msg *message_entity.Message) error

	GetHistoryMessage(
		ctx context.Context,
		conversationId string,
		minSeq int64,
		limit int,
	) ([]*message_entity.Message, error)

	GetLatestMessagesByConversationIDs(
		ctx context.Context,
		conversationIDs []string,
	) ([]*message_entity.Message, error)

	WithTx(tx any) MessageRepository
}
