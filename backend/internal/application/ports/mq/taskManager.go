package mq

import (
	"IM_backend/internal/shared/protocol"
	"context"
)

type TaskManager interface {
	HandleSendMessage(ctx context.Context, topic string, key string, event protocol.MessageEvent) error
	HandleReadMessageAck(ctx context.Context, topic string, key string, event protocol.MessageReadAckEvent) error
	HandleConversationSync(ctx context.Context, topic string, key string, event protocol.ConversationSyncSeqEvent) error
}
