package entity

import (
	messagevo "IM_backend/internal/domain/message/value_object"
	"errors"
	"time"
)

var ErrDuplicateClientMessage = errors.New("客户端消息已存在")
var ErrClientMessageConflict = errors.New("客户端消息标识对应的内容不一致")

type Message struct {
	MessageId      string
	ConversationId string
	SenderId       string
	ClientMsgId    *string
	RequestHash    string
	Seq            int64
	Type           messagevo.CType
	Content        string
	Status         messagevo.Status
	SendTime       int64
}

func NewMessage(
	messageId,
	conversationId,
	senderId string,
	seq int64,
	cType messagevo.CType,
	content string,
) *Message {
	return &Message{
		MessageId:      messageId,
		ConversationId: conversationId,
		Seq:            seq,
		Type:           cType,
		Content:        content,
		Status:         messagevo.Normal,
		SenderId:       senderId,
		SendTime:       time.Now().UnixMilli(),
	}
}
