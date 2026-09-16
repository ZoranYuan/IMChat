package message

import (
	messageentity "IM_backend/internal/domain/message/entity"
	"context"
)

type MessageRepository interface {
	CreateNewMessages(ctx context.Context, msgs []*messageentity.Message) error
	CreateNewMessage(ctx context.Context, msg *messageentity.Message) error
	FindByMessageID(ctx context.Context, messageID string) (*messageentity.Message, error)
	FindByClientMsgID(ctx context.Context, senderID, clientMsgID string) (*messageentity.Message, error)

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
	) ([]*messageentity.Message, error)

	ListBySeqs(
		ctx context.Context,
		conversationId string,
		seqs []int64,
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

type MessageStickerRepository interface {
	Create(ctx context.Context, item *messageentity.MessageSticker) error
	GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageSticker, error)
	BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageSticker, error)
	WithTx(tx any) MessageStickerRepository
}
