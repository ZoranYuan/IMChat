package message

import (
	messageentity "IM_backend/internal/domain/message/entity"
	"context"
)

type MessageRepository interface {

	// 保存消息
	Save(ctx context.Context, msg *messageentity.Message) error

	GetHistoryMessage(
		ctx context.Context,
		conversationId string,
		minSeq int64,
		limit int,
	) ([]*messageentity.Message, error)

	GetMessagesBySendTime(
		ctx context.Context,
		conversationId string,
		startTime int64,
		endTime int64,
		limit int,
	) ([]*messageentity.Message, error)

	GetDanmakuByRoomVideo(
		ctx context.Context,
		conversationId string,
		videoId string,
		startTime int64,
		endTime int64,
		limit int,
	) ([]*messageentity.Message, error)

	GetRoomVideoHistory(
		ctx context.Context,
		conversationId string,
		limit int,
	) ([]*messageentity.Message, error)

	CountRoomVideoMessages(
		ctx context.Context,
		conversationId string,
		videoId string,
	) (int64, error)

	GetLatestMessagesByConversationIDs(
		ctx context.Context,
		conversationIDs []string,
	) ([]*messageentity.Message, error)

	WithTx(tx any) MessageRepository
}
