package message

import (
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func toMessageDomain(m *model.Message) *messageentity.Message {
	if m == nil {
		return nil
	}

	return &messageentity.Message{
		MessageId:      m.MessageId,
		ConversationId: m.ConversationId,
		SenderId:       m.SenderId,
		ClientMsgId:    m.ClientMsgId,
		RequestHash:    m.RequestHash,
		Seq:            m.Seq,
		Type:           messagevo.CType(m.Type),
		Content:        m.Content,
		VideoId:        m.VideoId,
		VideoTime:      m.VideoTime,
		Status:         messagevo.Status(m.Status),
		SendTime:       m.SendTime,
	}
}

func toMessageModel(d *messageentity.Message) *model.Message {
	if d == nil {
		return nil
	}

	return &model.Message{
		MessageId:      d.MessageId,
		ConversationId: d.ConversationId,
		SenderId:       d.SenderId,
		ClientMsgId:    d.ClientMsgId,
		RequestHash:    d.RequestHash,
		Seq:            d.Seq,
		Type:           int8(d.Type),
		Content:        d.Content,
		VideoId:        d.VideoId,
		VideoTime:      d.VideoTime,
		Status:         int8(d.Status),
		SendTime:       d.SendTime,
	}
}
