package message_entity

import (
	message_valueobject "IM_backend/internal/domain/message/value_object"
	"time"
)

type Message struct {
	MessageId      string
	ConversationId string

	SendId string

	// 当前消息的自增序列值
	Seq int64
	// 消息状态
	Type    message_valueobject.CType
	Content string
	// 消息状态
	VideoTime int64
	Status    message_valueobject.Status

	SendTime int64
}

func BuildMessage(messageId, conversationId, sendId string, seq int64, content string, videoTime int64,
	cType message_valueobject.CType,
) *Message {
	return &Message{
		MessageId:      messageId,
		ConversationId: conversationId,
		Seq:            seq,
		Type:           cType,
		Content:        content,
		VideoTime:      videoTime,
		Status:         message_valueobject.Normal,
		SendId:         sendId,
		SendTime:       time.Now().Unix(),
	}
}
