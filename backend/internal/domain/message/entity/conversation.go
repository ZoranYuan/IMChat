package entity

import (
	messagevo "IM_backend/internal/domain/message/value_object"
)

type Conversation struct {
	ConversationId  string
	Convtype        messagevo.ConvType
	UserId1         string
	UserId2         string
	RoomId          string
	LatestMessageId string
	LatestSeq       int64
}

func NewConversation(
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

	if messagevo.ConvType(convTye) == messagevo.RoomChat {
		roomId = recvId
	} else {
		user2Id = recvId
	}

	return &Conversation{
		ConversationId:  conversationId,
		Convtype:        messagevo.ConvType(convTye), // 或根据业务
		UserId1:         sendId,
		UserId2:         user2Id, // 单聊需要
		RoomId:          roomId,
		LatestMessageId: latestMessageId,
		LatestSeq:       seq,
	}
}

func GetConversationID(sendId, targetId string, convType int) string {
	if convType == int(messagevo.PrivateChat) {
		return max(targetId, sendId) + "_" + min(targetId, sendId)
	}

	// group chat
	return targetId
}
