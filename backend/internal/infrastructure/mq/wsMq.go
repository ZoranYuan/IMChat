package mq

import (
	"IM_backend/internal/apis/ws"
	mq_interface "IM_backend/internal/infrastructure/mq/interface"
	"context"
	"encoding/json"
)

type Producer struct {
	gateway      *ws.GetWay
	roomResolver mq_interface.RoomResolver
}

func NewProducer(g *ws.GetWay, roomResolver mq_interface.RoomResolver) *Producer {
	return &Producer{
		gateway:      g,
		roomResolver: roomResolver,
	}
}

func (p *Producer) handleChat(msg *ws.MessageResData) error {
	// 私聊：直接投递给对方
	p.gateway.SendMessage("chat", msg)
	return nil
}

func (p *Producer) handleRoomChat(ctx context.Context, msg *ws.MessageResData) error {
	memberIds, err := p.roomResolver.GetRoomMembers(ctx, msg.RecvId)

	if err != nil {
		return err
	}

	for _, uid := range memberIds {
		p.gateway.SendMessage("chat", &ws.MessageResData{
			MessageId:      msg.MessageId,
			ConversationId: msg.ConversationId,
			RecvId:         uid,
			CType:          msg.CType,
			Content:        msg.Content,
			SendTime:       msg.SendTime,
		})
	}

	return nil
}

func (p *Producer) dispatch(ctx context.Context, topic string, key string, msg *ws.MessageResData) error {
	switch topic {

	case "private_chat":
		return p.handleChat(msg)

	case "room_chat":
		return p.handleRoomChat(ctx, msg)

	default:
		return nil
	}
}

func (p *Producer) SendMessage(ctx context.Context, topic string, key string, payload []byte) error {
	var msg ws.MessageResData

	if err := json.Unmarshal(payload, &msg); err != nil {
		return err
	}

	return p.dispatch(ctx, topic, key, &msg)
}
