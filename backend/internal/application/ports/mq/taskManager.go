package mq

import (
	"IM_backend/internal/shared/protocol"
	"context"
)

type TaskManager interface {
	SendMessage(ctx context.Context, topic string, key string, event protocol.MessageEvent) error
	SendMessageReadAck(ctx context.Context, topic string, key string, event protocol.MessageReadAckEvent) error
	SendConversationSyncSeq(ctx context.Context, topic string, key string, event protocol.ConversationSyncSeqEvent) error
}
