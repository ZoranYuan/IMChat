package mq

import (
	mqclient "IM_backend/internal/infrastructure/messaging/client"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
)

type TaskManager struct {
	client mqclient.Client
}

func NewTaskManager(c mqclient.Client) *TaskManager {
	return &TaskManager{
		client: c,
	}
}

func (t *TaskManager) handleMessage(ctx context.Context, topic string, key string, event protocol.MessageEvent) error {
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

	return t.client.SendMessage(ctx, topic, key, data)
}

func (t *TaskManager) handleMessageReadAck(ctx context.Context, topic string, key string, event protocol.MessageReadAckEvent) error {
	var payload []byte

	payload, err := json.Marshal(event)
	if err != nil {
		// 补偿措施
	}

	var envelope = protocol.Envelope{
		To:      event.UserId,
		Payload: payload,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		// 补偿措施
	}

	return t.client.SendMessage(ctx, topic, key, data)
}

func (t *TaskManager) handleConversationSyncSeq(ctx context.Context, topic string, key string, event protocol.ConversationSyncSeqEvent) error {
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

	return t.client.SendMessage(ctx, topic, key, data)
}

func (t *TaskManager) dispatch(ctx context.Context, topic string, key string, event protocol.Event) error {
	switch event.Type {
	case protocol.EventTypeMessage:
		// msg
		var message protocol.MessageEvent
		if err := json.Unmarshal(event.Data, &message); err != nil {
			return err
		}
		return t.handleMessage(ctx, topic, key, message)
	case protocol.EventMessageReadAck:
		var message protocol.MessageReadAckEvent
		if err := json.Unmarshal(event.Data, &message); err != nil {
			return err
		}
		return t.handleMessageReadAck(ctx, topic, key, message)
	case protocol.EventConversationSyncSeq:
		var syncEvent protocol.ConversationSyncSeqEvent
		if err := json.Unmarshal(event.Data, &syncEvent); err != nil {
			return err
		}
		return t.handleConversationSyncSeq(ctx, topic, key, syncEvent)
	}

	return nil
}

func (t *TaskManager) SendMessage(ctx context.Context, topic string, key string, event protocol.MessageEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return t.dispatch(ctx, topic, key, protocol.Event{
		Type: protocol.EventType(topic),
		Data: data,
	})
}

func (t *TaskManager) SendHistoryMessageAck(ctx context.Context, topic string, key string, event protocol.MessageReadAckEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return t.dispatch(ctx, topic, key, protocol.Event{
		Type: protocol.EventMessageReadAck,
		Data: data,
	})
}

func (t *TaskManager) SendConversationSyncSeq(ctx context.Context, topic string, key string, event protocol.ConversationSyncSeqEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return t.dispatch(ctx, topic, key, protocol.Event{
		Type: protocol.EventConversationSyncSeq,
		Data: data,
	})
}
