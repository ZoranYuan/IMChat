package mq

import (
	mq_client "IM_backend/internal/infrastructure/mq/client"
	conversation_port "IM_backend/internal/port/conversation"
	"IM_backend/internal/protocol"
	"context"
	"encoding/json"
)

type TaskManager struct {
	client            mq_client.Client
	conversationCache conversation_port.ConversationCacheInterface
}

func NewTaskManager(c mq_client.Client, conversationCache conversation_port.ConversationCacheInterface) *TaskManager {
	return &TaskManager{
		client:            c,
		conversationCache: conversationCache,
	}
}

func (t *TaskManager) handleChat(topic string, key string, event protocol.MessageEvent) error {
	// 私聊：直接投递给对方
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

	return t.client.SendMessage(topic, key, data)
}

func (t *TaskManager) handleRoomChat(ctx context.Context, topic string, key string, event protocol.MessageEvent) error {
	memberIds, err := t.conversationCache.GetMembers(ctx, key)

	if err != nil {
		return err
	}

	for _, uid := range memberIds {
		var payload []byte

		payload, err := json.Marshal(event)
		if err != nil {
			// 补偿措施
		}

		var envelope = protocol.Envelope{
			From:    event.SendId,
			To:      uid,
			Payload: payload,
		}

		data, err := json.Marshal(envelope)
		if err != nil {
			// 补偿措施
		}

		if err := t.client.SendMessage(topic, key, data); err != nil {
			// 补偿措施
		}
	}

	return nil
}

func (t *TaskManager) handleHistoryRead(ctx context.Context, topic string, key string, event protocol.MessageReadAckEvent) error {
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

	return t.client.SendMessage(topic, key, data)
}

func (t *TaskManager) dispatch(ctx context.Context, topic string, key string, event protocol.Event) error {
	switch event.Type {
	case protocol.EventTypeMessage:
		var message protocol.MessageEvent

		var err error
		if err := json.Unmarshal(event.Data, &message); err != nil {
			return err
		}

		if message.ConvType == protocol.PrivateChat {
			err = t.handleChat(topic, key, message)
		} else {
			err = t.handleRoomChat(ctx, topic, key, message)
		}

		return err
	case protocol.EventMessageReadAck:

	}

	return nil
}

func (t *TaskManager) SendMessage(ctx context.Context, todic string, key string, event protocol.MessageEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return t.dispatch(ctx, todic, key, protocol.Event{
		Type: protocol.EventType(todic),
		Data: data,
	})
}

func (t *TaskManager) SendHistoryMessageAck(ctx context.Context, todic string, key string, event protocol.MessageReadAckEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return t.dispatch(ctx, todic, key, protocol.Event{
		Type: protocol.EventMessageReadAck,
		Data: data,
	})
}
