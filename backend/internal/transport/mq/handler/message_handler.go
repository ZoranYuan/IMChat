package handler

import (
	messageapp "IM_backend/internal/application/message"
	eventbus "IM_backend/internal/application/ports/eventbus"
	"IM_backend/internal/shared/diagnostics"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"time"
)

type MessageDelivery interface {
	Deliver(
		ctx context.Context,
		eventType string,
		conversationID string,
		envelope protocol.Envelope,
		event protocol.MessageEvent,
	) error
}

type messageDeliveryCloser interface {
	Close(ctx context.Context)
}

type MessageHandler struct {
	delivery MessageDelivery
}

func NewMessageHandler(delivery MessageDelivery) *MessageHandler {
	return &MessageHandler{
		delivery: delivery,
	}
}

func (handler *MessageHandler) Close(ctx context.Context) {
	if closer, ok := handler.delivery.(messageDeliveryCloser); ok {
		closer.Close(ctx)
	}
}

func (handler *MessageHandler) Handle(ctx context.Context, message eventbus.IncomingEvent) error {
	startedAt := time.Now()
	envelope, err := decodeEnvelope(message.Payload)
	if err != nil {
		return eventbus.NonRetryable(err)
	}

	var messageEvent protocol.MessageEvent
	if err := json.Unmarshal(envelope.Payload, &messageEvent); err != nil {
		return eventbus.NonRetryable(err)
	}

	decodeDuration := time.Since(startedAt)
	deliveryStartedAt := time.Now()
	err = handler.delivery.Deliver(ctx, string(message.Name), string(message.Key), envelope, messageEvent)
	deliveryDuration := time.Since(deliveryStartedAt)
	eventAge := time.Duration(0)
	if messageEvent.SendTime > 0 {
		eventAge = time.Since(time.UnixMilli(messageEvent.SendTime))
	}
	diagnostics.Logf("stage=message_handler event_id=%s client_msg_id=%s message_id=%s conversation_id=%s seq=%d event_age_ms=%d decode_us=%d delivery_us=%d total_us=%d outcome=%s",
		message.EventID, messageEvent.ClientMsgId, messageEvent.MessageId, messageEvent.ConversationId, messageEvent.Seq, eventAge.Milliseconds(), decodeDuration.Microseconds(),
		deliveryDuration.Microseconds(), time.Since(startedAt).Microseconds(), handlerOutcome(err))
	if errors.Is(err, messageapp.ErrUnknownConversationType) {
		return eventbus.NonRetryable(err)
	}
	return err
}

func handlerOutcome(err error) string {
	if err != nil {
		return "error"
	}
	return "delivered"
}

func decodeEnvelope(data []byte) (protocol.Envelope, error) {
	var envelope protocol.Envelope
	err := json.Unmarshal(data, &envelope)
	return envelope, err
}
