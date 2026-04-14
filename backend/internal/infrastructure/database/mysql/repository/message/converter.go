package message_repository

import (
	message_entity "IM_backend/internal/domain/message/entity"
	message_valueobject "IM_backend/internal/domain/message/value_object"
	"IM_backend/internal/infrastructure/database/mysql/model"
)

func toMessageDomain(m *model.Message) *message_entity.Message {
	if m == nil {
		return nil
	}

	return &message_entity.Message{
		MessageId:      m.MessageId,
		ConversationId: m.ConversationId,
		SendId:         m.SendId,
		Seq:            m.Seq,
		Type:           message_valueobject.CType(m.Type),
		Content:        m.Content,
		VideoTime:      m.VideoTime,
		Status:         message_valueobject.Status(m.Status),
		SendTime:       m.SendTime,
	}
}

func toMessageModel(d *message_entity.Message) *model.Message {
	if d == nil {
		return nil
	}

	return &model.Message{
		MessageId:      d.MessageId,
		ConversationId: d.ConversationId,
		SendId:         d.SendId,
		Seq:            d.Seq,
		Type:           int8(d.Type),
		Content:        d.Content,
		VideoTime:      d.VideoTime,
		Status:         int8(d.Status),
		SendTime:       d.SendTime,
	}
}

func toUserConversationDomain(u *model.UserConversation) *message_entity.UserConversation {
	if u == nil {
		return nil
	}

	return &message_entity.UserConversation{
		UserId:         u.UserId,
		ConversationId: u.ConversationId,
		LastReadSeq:    u.LastReadSeq,
		LatestSyncSeq:  u.LatestSyncSeq,
		IsMuted:        u.IsMuted,
	}
}

func toUserConversationModel(d *message_entity.UserConversation) *model.UserConversation {
	if d == nil {
		return nil
	}

	return &model.UserConversation{
		UserId:         d.UserId,
		ConversationId: d.ConversationId,
		LastReadSeq:    d.LastReadSeq,
		LatestSyncSeq:  d.LatestSyncSeq,
		IsMuted:        d.IsMuted,
	}
}

func toConversationDomain(c *model.Conversation) *message_entity.Conversation {
	if c == nil {
		return nil
	}

	return &message_entity.Conversation{
		ConversationId: c.ConversationId,
		Convtype:       message_valueobject.ConvType(c.Convtype),
		UserId1:        c.UserId1,
		UserId2:        c.UserId2,
		RoomId:         c.RoomId,
		LastSeq:        c.LatestSeq,
	}
}

func toConversationModel(d *message_entity.Conversation) *model.Conversation {
	if d == nil {
		return nil
	}

	return &model.Conversation{
		ConversationId: d.ConversationId,
		Convtype:       int8(d.Convtype),
		UserId1:        d.UserId1,
		UserId2:        d.UserId2,
		RoomId:         d.RoomId,
		LatestSeq:      d.LastSeq,
	}
}
