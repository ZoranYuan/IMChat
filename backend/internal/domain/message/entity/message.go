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
	// 弹幕/视频关联（非媒体类型字段，用于文本消息关联视频时间戳）
	VideoId   string
	VideoTime *int64
	Status    messagevo.Status
	SendTime  int64
}

func NewMessage(
	messageId,
	conversationId,
	sendId string,
	seq int64,
	cType messagevo.CType,
	content string,
	videoId string,
	videoTime *int64,
) *Message {
	return &Message{
		MessageId:      messageId,
		ConversationId: conversationId,
		Seq:            seq,
		Type:           cType,
		Content:        content,
		VideoId:        videoId,
		VideoTime:      videoTime,
		Status:         messagevo.Normal,
		SendId:         sendId,
		SendTime:       time.Now().UnixMilli(),
	}
}
