package https_message

import (
	application_message "IM_backend/internal/applications/message"
)

type MessageHistoryReq struct {
	ConversationId string `form:"conversationId" binding:"required"`
	Cursor         int64  `form:"cursor"`
	Limit          int    `form:"limit"`
}

type Message struct {
	MessageId string `json:"messageId"`
	SenderId  string `json:"senderId"`
	Seq       int64  `json:"seq"`
	ConvType  int    `json:"convType"` // 单聊/群聊
	CType     int    `json:"cType"`    // 消息类型（文本/图片等）
	Content   string `json:"content"`
	SendTime  int64  `json:"sendTime"`
}

type OfflineMessageRes struct {
	ConversationId string  `json:"conversationId"`
	UnRead         int64   `json:"unread"`
	LatestMessage  Message `json:"latestMessage"`
}

type MessageHistoryRes struct {
	Messages   []Message `json:"messages"`
	NextCursor int64     `json:"nextCursor"`
	HasMore    bool      `json:"hasMore"`
}

func toHistoryMessageRes(messages []application_message.MessageAppeDTO, nextCursor int64, hashMore bool) (res MessageHistoryRes) {
	ms := make([]Message, 0, len(messages))

	for _, m := range messages {
		message := Message{
			MessageId: m.MessageId,
			SenderId:  m.SendId,
			Seq:       m.Seq,
			ConvType:  m.ConvType,
			CType:     m.CType,
			Content:   m.Content,
			SendTime:  m.SendTime,
		}

		ms = append(ms, message)
	}

	res.Messages = ms
	res.HasMore = hashMore
	res.NextCursor = nextCursor

	return
}

func toOfflineMessageRes(messages []application_message.MessageAppeDTO, unreadMap map[string]int64) (res []OfflineMessageRes) {
	for _, m := range messages {
		latestMessage := Message{
			MessageId: m.MessageId,
			SenderId:  m.SendId,
			Seq:       m.Seq,
			ConvType:  m.ConvType,
			CType:     m.CType,
			Content:   m.Content,
			SendTime:  m.SendTime,
		}

		om := OfflineMessageRes{
			ConversationId: m.ConsersationId,
			UnRead:         unreadMap[m.ConsersationId],
			LatestMessage:  latestMessage,
		}

		res = append(res, om)
	}

	return
}
