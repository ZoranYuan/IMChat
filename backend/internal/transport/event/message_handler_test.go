package event

import (
	messageapp "IM_backend/internal/application/message"
	eventbus "IM_backend/internal/application/ports/eventbus"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type deliveryStub struct {
	eventType string
	convID    string
	envelope  protocol.Envelope
	event     protocol.MessageEvent
	err       error
}

func (stub *deliveryStub) Deliver(
	_ context.Context,
	eventType string,
	conversationID string,
	envelope protocol.Envelope,
	event protocol.MessageEvent,
) error {
	stub.eventType = eventType
	stub.convID = conversationID
	stub.envelope = envelope
	stub.event = event
	return stub.err
}

func TestMessageHandlerDecodesEnvelope(t *testing.T) {
	event := protocol.MessageEvent{MessageId: "m1", ConvType: protocol.PrivateChat, Seq: 2}
	eventPayload, _ := json.Marshal(event)
	payload, _ := json.Marshal(protocol.Envelope{From: "u1", To: "u2", Payload: eventPayload})
	stub := &deliveryStub{}
	handler := NewMessageHandler(stub)
	defer handler.Close(context.Background())

	err := handler.Handle(context.Background(), eventbus.IncomingEvent{
		Name:    protocol.EventTypeSendMessage,
		Key:     []byte("c1"),
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("处理消息失败：%v", err)
	}
	if stub.eventType != protocol.EventTypeSendMessage || stub.convID != "c1" {
		t.Fatalf("投递事件元数据错误：event=%s conv=%s", stub.eventType, stub.convID)
	}
	if stub.envelope.From != "u1" || stub.envelope.To != "u2" {
		t.Fatalf("投递 envelope 错误：%+v", stub.envelope)
	}
	if stub.event.MessageId != "m1" || stub.event.Seq != 2 {
		t.Fatalf("投递事件载荷错误：%+v", stub.event)
	}
}

func TestMessageHandlerMarksMalformedPayloadPermanent(t *testing.T) {
	handler := NewMessageHandler(&deliveryStub{})
	defer handler.Close(context.Background())
	err := handler.Handle(context.Background(), eventbus.IncomingEvent{Payload: []byte("not-json")})

	var permanentError *eventbus.NonRetryableError
	if !errors.As(err, &permanentError) {
		t.Fatalf("非法消息应标记为永久错误：%v", err)
	}
}

func TestMessageHandlerMarksUnknownConversationTypePermanent(t *testing.T) {
	event := protocol.MessageEvent{MessageId: "m1", ConvType: 99, Seq: 2}
	eventPayload, _ := json.Marshal(event)
	payload, _ := json.Marshal(protocol.Envelope{From: "u1", To: "u2", Payload: eventPayload})
	handler := NewMessageHandler(&deliveryStub{err: messageapp.ErrUnknownConversationType})
	defer handler.Close(context.Background())

	err := handler.Handle(context.Background(), eventbus.IncomingEvent{
		Name:    protocol.EventTypeSendMessage,
		Key:     []byte("c1"),
		Payload: payload,
	})
	var permanentError *eventbus.NonRetryableError
	if !errors.As(err, &permanentError) {
		t.Fatalf("未知会话类型应标记为永久错误：%v", err)
	}
}
