package message

import (
	messageentity "IM_backend/internal/domain/message/entity"
	"context"
)

type MessageRepository interface {
	CreateNewMessages(ctx context.Context, msgs []*messageentity.Message) error
	CreateNewMessage(ctx context.Context, msg *messageentity.Message) error
	FindByClientMsgID(ctx context.Context, sendID, clientMsgID string) (*messageentity.Message, error)

	GetHistoryMessage(
		ctx context.Context,
		conversationId string,
		minSeq int64,
		limit int,
	) ([]*messageentity.Message, error)

	ListAfterSeq(
		ctx context.Context,
		conversationId string,
		afterSeq int64,
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

	ListDistinctSendersBySeqRange(
		ctx context.Context,
		conversationId string,
		minSeqExclusive int64,
		maxSeqInclusive int64,
		excludeUserId string,
	) ([]string, error)

	WithTx(tx any) MessageRepository
}

type MessageImageRepository interface {
	Create(ctx context.Context, item *messageentity.MessageImage) error
	GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageImage, error)
	BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageImage, error)
	WithTx(tx any) MessageImageRepository
}

type MessageFileRepository interface {
	Create(ctx context.Context, item *messageentity.MessageFile) error
	GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageFile, error)
	BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageFile, error)
	WithTx(tx any) MessageFileRepository
}

type MessageStickerRepository interface {
	Create(ctx context.Context, item *messageentity.MessageSticker) error
	GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageSticker, error)
	BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageSticker, error)
	WithTx(tx any) MessageStickerRepository
}

type MessageVideoRepository interface {
	Create(ctx context.Context, item *messageentity.MessageVideo) error
	GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageVideo, error)
	BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageVideo, error)
	WithTx(tx any) MessageVideoRepository
}
