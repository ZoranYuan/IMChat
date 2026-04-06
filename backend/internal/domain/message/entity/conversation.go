package message_entity

import (
	message_valueobject "IM_backend/internal/domain/message/value_object"
)

type Conversation struct {
	ConversationId string

	Type message_valueobject.ConvType

	UserId1 string
	UserId2 string

	RoomId string

	// 当前会话的最大 seq
	LastSeq int64
}

func NewConversation(conversationId, userId, recvId string, convTye int) *Conversation {

	var (
		user2Id string
		roomId  string
	)

	if message_valueobject.ConvType(convTye) == message_valueobject.RoomChat {
		roomId = recvId
	} else {
		user2Id = recvId
	}

	return &Conversation{
		ConversationId: conversationId,
		Type:           message_valueobject.ConvType(convTye), // 或根据业务
		UserId1:        userId,
		UserId2:        user2Id, // 单聊需要
		RoomId:         roomId,
		LastSeq:        0,
	}
}
