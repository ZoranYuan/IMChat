package mq

import (
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
)

type Client interface {
	ProduceMessage(context.Context, string, string, []byte) error
}

type TaskManager struct {
	client Client
}

func NewTaskManager(c Client) *TaskManager {
	return &TaskManager{
		client: c,
	}
}

func (t *TaskManager) handleSendMessage(ctx context.Context, topic string, key string, event protocol.MessageEvent) error {
	var payload []byte

	payload, err := json.Marshal(event)
	if err != nil {
		// 补偿措施
	}

	var envelope = protocol.Envelope{
		From:    event.SendId,
		To:      event.RecvId,
		Payload: payload,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		// 补偿措施
	}

	return t.client.ProduceMessage(ctx, topic, key, data)
}

func (t *TaskManager) handleReadMessageAck(ctx context.Context, topic string, key string, event protocol.MessageReadAckEvent) error {
	var payload []byte

	payload, err := json.Marshal(event)
	if err != nil {
		// 补偿措施
	}

	var envelope = protocol.Envelope{
		To:      event.ConversationId,
		Payload: payload,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		// 补偿措施
	}

	return t.client.ProduceMessage(ctx, topic, key, data)
}

func (t *TaskManager) handleConversationSync(ctx context.Context, topic string, key string, event protocol.ConversationSyncSeqEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	envelope := protocol.Envelope{
		Payload: payload,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	return t.client.ProduceMessage(ctx, topic, key, data)
}

func (t *TaskManager) dispatch(ctx context.Context, topic string, key string, event protocol.Event) error {
	switch event.Type {
	case protocol.EventTypeSendMessage:
		var message protocol.MessageEvent
		if err := json.Unmarshal(event.Data, &message); err != nil {
			return err
		}
		return t.handleSendMessage(ctx, topic, key, message)
	case protocol.EventReadMessageAck:
		var message protocol.MessageReadAckEvent
		if err := json.Unmarshal(event.Data, &message); err != nil {
			return err
		}
		return t.handleReadMessageAck(ctx, topic, key, message)
	case protocol.EventConversationSyncSeq:
		var syncEvent protocol.ConversationSyncSeqEvent
		if err := json.Unmarshal(event.Data, &syncEvent); err != nil {
			return err
		}
		return t.handleConversationSync(ctx, topic, key, syncEvent)
	}

	return nil
}

func (t *TaskManager) HandleSendMessage(ctx context.Context, topic string, key string, event protocol.MessageEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return t.dispatch(ctx, topic, key, protocol.Event{
		Type: protocol.EventType(topic),
		Data: data,
	})
}

func (t *TaskManager) HandleReadMessageAck(ctx context.Context, topic string, key string, event protocol.MessageReadAckEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return t.dispatch(ctx, topic, key, protocol.Event{
		Type: protocol.EventReadMessageAck,
		Data: data,
	})
}

func (t *TaskManager) HandleConversationSync(ctx context.Context, topic string, key string, event protocol.ConversationSyncSeqEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return t.dispatch(ctx, topic, key, protocol.Event{
		Type: protocol.EventConversationSyncSeq,
		Data: data,
	})
}
