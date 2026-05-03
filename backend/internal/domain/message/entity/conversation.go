package message_entity

import (
	message_valueobject "IM_backend/internal/domain/message/value_object"
)

type Conversation struct {
	ConversationId  string
	Convtype        message_valueobject.ConvType
	UserId1         string
	UserId2         string
	RoomId          string
	LatestMessageId string
	LatestSeq       int64
}

func BuildConversation(
	conversationId,
	sendId, recvId string,
	convTye int,
	seq int64,
	latestMessageId string,
) *Conversation {
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
		ConversationId:  conversationId,
		Convtype:        message_valueobject.ConvType(convTye), // 或根据业务
		UserId1:         sendId,
		UserId2:         user2Id, // 单聊需要
		RoomId:          roomId,
		LatestMessageId: latestMessageId,
		LatestSeq:       seq,
	}
}

func GetConversationID(sendId, targetId string, convType int) string {
	if convType == int(message_valueobject.PrivateChat) {
		return max(targetId, sendId) + "_" + min(targetId, sendId)
	}

	// group chat
	return targetId
}
