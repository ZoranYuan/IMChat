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
