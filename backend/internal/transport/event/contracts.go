package event

import (
	messageapp "IM_backend/internal/application/message"
	"IM_backend/internal/shared/protocol"
	"context"
)

type MessageDelivery interface {
	DeliverMessage(ctx context.Context, command messageapp.DeliveryCommand) error
	DeliverReadNotification(ctx context.Context, event protocol.MessageReadCommittedEvent) error
}
