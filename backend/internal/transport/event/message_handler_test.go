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
	command messageapp.DeliveryCommand
}

func (stub *deliveryStub) DeliverMessage(_ context.Context, command messageapp.DeliveryCommand) error {
	stub.command = command
	return nil
}

func (stub *deliveryStub) DeliverReadNotification(context.Context, protocol.MessageReadAckEvent, []byte) error {
	return nil
}

func TestMessageHandlerDecodesEnvelope(t *testing.T) {
	event := protocol.MessageEvent{MessageId: "m1", ConvType: protocol.PrivateChat, Seq: 2}
	eventPayload, _ := json.Marshal(event)
	payload, _ := json.Marshal(protocol.Envelope{From: "u1", To: "u2", Payload: eventPayload})
	stub := &deliveryStub{}
	handler := NewMessageHandler(stub)

	err := handler.Handle(context.Background(), eventbus.IncomingEvent{
		Name:    protocol.EventTypeSendMessage,
		Key:     []byte("c1"),
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("处理消息失败：%v", err)
	}
	if stub.command.ConversationId != "c1" || stub.command.Message.MessageId != "m1" {
		t.Fatalf("消息映射错误：%+v", stub.command)
	}
}

func TestMessageHandlerMarksMalformedPayloadPermanent(t *testing.T) {
	handler := NewMessageHandler(&deliveryStub{})
	err := handler.Handle(context.Background(), eventbus.IncomingEvent{Payload: []byte("not-json")})

	var permanentError *eventbus.NonRetryableError
	if !errors.As(err, &permanentError) {
		t.Fatalf("非法消息应标记为永久错误：%v", err)
	}
}
