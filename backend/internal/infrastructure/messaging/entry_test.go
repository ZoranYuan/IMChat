package mq

import (
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"testing"
)

type fakeMQClient struct {
	topic   string
	key     string
	payload []byte
}

func (c *fakeMQClient) SendMessage(ctx context.Context, topic string, key string, payload []byte) error {
	c.topic = topic
	c.key = key
	c.payload = payload
	return nil
}

func TestSendConversationSyncSeq(t *testing.T) {
	client := &fakeMQClient{}
	manager := NewTaskManager(client)

	event := protocol.ConversationSyncSeqEvent{
		Items: []protocol.ConversationSyncSeqItem{
			{
				UserId:         "u1",
				ConversationId: "conv-1",
				LatestSeq:      12,
			},
		},
	}

	err := manager.SendConversationSyncSeq(
		context.Background(),
		protocol.EventConversationSyncSeq,
		"u1",
		event,
	)
	if err != nil {
		t.Fatalf("SendConversationSyncSeq() error = %v", err)
	}
	if client.topic != protocol.EventConversationSyncSeq || client.key != "u1" {
		t.Fatalf("unexpected message target topic=%q key=%q", client.topic, client.key)
	}

	var envelope protocol.Envelope
	if err := json.Unmarshal(client.payload, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	var got protocol.ConversationSyncSeqEvent
	if err := json.Unmarshal(envelope.Payload, &got); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].UserId != "u1" || got.Items[0].LatestSeq != 12 {
		t.Fatalf("unexpected sync event: %+v", got)
	}
}

func TestSendMessage(t *testing.T) {
	client := &fakeMQClient{}
	manager := NewTaskManager(client)

	event := protocol.MessageEvent{
		MessageId:      "msg-1",
		ConversationId: "conv-1",
		SendId:         "u1",
		RecvId:         "u2",
		Seq:            9,
		ConvType:       protocol.PrivateChat,
		CType:          1,
		Content:        "hello",
		SendTime:       1710000001000,
		ClientMsgId:    "client-1",
	}

	err := manager.SendMessage(context.Background(), protocol.EventTypeMessage, "conv-1", event)
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if client.topic != protocol.EventTypeMessage || client.key != "conv-1" {
		t.Fatalf("unexpected message target topic=%q key=%q", client.topic, client.key)
	}

	var envelope protocol.Envelope
	if err := json.Unmarshal(client.payload, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.From != "u1" || envelope.To != "u2" {
		t.Fatalf("unexpected envelope route: %+v", envelope)
	}

	var got protocol.MessageEvent
	if err := json.Unmarshal(envelope.Payload, &got); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}
	if got.MessageId != event.MessageId || got.ConversationId != event.ConversationId || got.SendId != event.SendId || got.RecvId != event.RecvId {
		t.Fatalf("unexpected message event: %+v", got)
	}
}

func TestPublishMessageReadAck(t *testing.T) {
	client := &fakeMQClient{}
	manager := NewTaskManager(client)

	event := protocol.MessageReadAckEvent{
		UserId:         "u2",
		ConversationId: "conv-1",
		LastReadSeq:    18,
		ConvType:       protocol.RoomChat,
		SenderId:       "u1",
		Avatar:         "avatar-u2",
	}

	err := manager.PublishMessageReadAck(context.Background(), protocol.EventMessageReadAck, "u2:conv-1", event)
	if err != nil {
		t.Fatalf("PublishMessageReadAck() error = %v", err)
	}
	if client.topic != protocol.EventMessageReadAck || client.key != "u2:conv-1" {
		t.Fatalf("unexpected message target topic=%q key=%q", client.topic, client.key)
	}

	var envelope protocol.Envelope
	if err := json.Unmarshal(client.payload, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.To != "conv-1" {
		t.Fatalf("unexpected envelope target: %+v", envelope)
	}

	var got protocol.MessageReadAckEvent
	if err := json.Unmarshal(envelope.Payload, &got); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}
	if got.UserId != event.UserId || got.ConversationId != event.ConversationId || got.LastReadSeq != event.LastReadSeq || got.Avatar != event.Avatar {
		t.Fatalf("unexpected read ack event: %+v", got)
	}
}
