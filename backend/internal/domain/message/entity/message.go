package entity

import (
	messagevo "IM_backend/internal/domain/message/value_object"
	"time"
)

type Message struct {
	MessageId      string
	ConversationId string
	SendId         string
	Seq            int64
	Type           messagevo.CType
	Content        string
	VideoTime      *int64
	Status         messagevo.Status
	SendTime       int64
}

func BuildMessage(messageId, conversationId, sendId string, seq int64, content string, videoTime *int64,
	cType messagevo.CType,
) *Message {
	return &Message{
		MessageId:      messageId,
		ConversationId: conversationId,
		Seq:            seq,
		Type:           cType,
		Content:        content,
		VideoTime:      videoTime,
		Status:         messagevo.Normal,
		SendId:         sendId,
		SendTime:       time.Now().UnixMilli(),
	}
}
