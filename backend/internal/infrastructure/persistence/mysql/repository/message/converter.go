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
		SendId:         m.SendId,
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
		SendId:         d.SendId,
		Seq:            d.Seq,
		Type:           int8(d.Type),
		Content:        d.Content,
		VideoId:        d.VideoId,
		VideoTime:      d.VideoTime,
		Status:         int8(d.Status),
		SendTime:       d.SendTime,
	}
}

func toUserConversationDomain(u *model.UserConversation) *messageentity.UserConversation {
	if u == nil {
		return nil
	}

	return &messageentity.UserConversation{
		UserId:         u.UserId,
		ConversationId: u.ConversationId,
		LastReadSeq:    u.LastReadSeq,
		LatestSyncSeq:  u.LatestSyncSeq,
		IsMuted:        u.IsMuted,
	}
}

func toUserConversationModel(d *messageentity.UserConversation) *model.UserConversation {
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

func toConversationDomain(c *model.Conversation) *messageentity.Conversation {
	if c == nil {
		return nil
	}

	return &messageentity.Conversation{
		ConversationId:  c.ConversationId,
		Convtype:        messagevo.ConvType(c.Convtype),
		UserId1:         c.UserId1,
		UserId2:         c.UserId2,
		RoomId:          c.RoomId,
		LatestSeq:       c.LatestSeq,
		LatestMessageId: c.LatestMessageId,
	}
}

func toConversationModel(d *messageentity.Conversation) *model.Conversation {
	if d == nil {
		return nil
	}

	return &model.Conversation{
		ConversationId:  d.ConversationId,
		Convtype:        int8(d.Convtype),
		UserId1:         d.UserId1,
		UserId2:         d.UserId2,
		RoomId:          d.RoomId,
		LatestSeq:       d.LatestSeq,
		LatestMessageId: d.LatestMessageId,
	}
}
