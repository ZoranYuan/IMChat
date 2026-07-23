package event

import (
	eventbus "IM_backend/internal/application/ports/eventbus"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type deliveryStub struct {
	eventType string
	userID    string
	payload   []byte
}

func (stub *deliveryStub) DeliverToUser(eventType, userID string, payload []byte) error {
	stub.eventType = eventType
	stub.userID = userID
	stub.payload = payload
	return nil
}

func TestMessageHandlerDecodesEnvelope(t *testing.T) {
	event := protocol.MessageEvent{MessageId: "m1", ConvType: protocol.PrivateChat, Seq: 2}
	eventPayload, _ := json.Marshal(event)
	payload, _ := json.Marshal(protocol.Envelope{From: "u1", To: "u2", Payload: eventPayload})
	stub := &deliveryStub{}
	handler := NewMessageHandler(stub, nil, nil, nil)

	err := handler.Handle(context.Background(), eventbus.IncomingEvent{
		Name:    protocol.EventTypeSendMessage,
		Key:     []byte("c1"),
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("处理消息失败：%v", err)
	}
	if stub.eventType != protocol.EventTypeSendMessage || stub.userID != "u2" {
		t.Fatalf("投递目标错误：event=%s user=%s", stub.eventType, stub.userID)
	}

	var got protocol.MessageEvent
	if err := json.Unmarshal(stub.payload, &got); err != nil {
		t.Fatalf("解析投递载荷失败：%v", err)
	}
	if got.MessageId != "m1" || got.Seq != 2 {
		t.Fatalf("投递载荷错误：%+v", got)
	}
}

func TestMessageHandlerMarksMalformedPayloadPermanent(t *testing.T) {
	handler := NewMessageHandler(&deliveryStub{}, nil, nil, nil)
	err := handler.Handle(context.Background(), eventbus.IncomingEvent{Payload: []byte("not-json")})

	var permanentError *eventbus.NonRetryableError
	if !errors.As(err, &permanentError) {
		t.Fatalf("非法消息应标记为永久错误：%v", err)
	}
}
