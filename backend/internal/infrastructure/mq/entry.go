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
	// 通过 conversationId 作为 key ，让同一个会话消息尽量落在同一个分区上，对于群聊，roomId 就是 conversationId
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

func (t *TaskManager) dispatch(ctx context.Context, topic string, key string, event protocol.MessageEvent) error {
	switch topic {

	case "chat":
		var err error
		if event.ConvType == 1 {
			err = t.handleChat(topic, key, event)
		} else {
			err = t.handleRoomChat(ctx, topic, key, event)
		}
		return err

	default:
		return nil
	}
}

func (t *TaskManager) SendMessage(ctx context.Context, todic string, key string, event protocol.MessageEvent) error {
	return t.dispatch(ctx, todic, key, event)
}
