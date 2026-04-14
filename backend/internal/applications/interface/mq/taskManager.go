package mq_interface

import (
	"IM_backend/internal/protocol"
	"context"
)

type TaskManager interface {
	SendMessage(ctx context.Context, todic string, key string, event protocol.MessageEvent) error
	SendHistoryMessageAck(ctx context.Context, todic string, key string, event protocol.HistoryMessageReadAckEvent) error
}
