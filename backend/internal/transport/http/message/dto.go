package message

import (
	messageapp "IM_backend/internal/application/message"
)

type MessageHistoryReq struct {
	ConversationId string `form:"conversationId" binding:"required"`
	Cursor         int64  `form:"cursor"`
	Limit          int    `form:"limit"`
}

type DanmakuReq struct {
	VideoId   string `form:"videoId" binding:"required"`
	StartTime int64  `form:"startTime"`
	EndTime   int64  `form:"endTime"`
	Limit     int    `form:"limit"`
}

type Message struct {
	MessageId      string `json:"messageId"`
	SenderId       string `json:"senderId"`
	SenderUsername string `json:"senderUsername"`
	Seq            int64  `json:"seq"`
	ConvType       int    `json:"convType"` // 单聊/群聊
	CType          int    `json:"cType"`    // 消息类型（文本/图片等）
	Content        string `json:"content"`
	SendTime       int64  `json:"sendTime"`
	VideoId        string `json:"videoId,omitempty"`
	VideoTime      *int64 `json:"videoTime,omitempty"`
}

type OfflineMessageRes struct {
	ConversationId string  `json:"conversationId"`
	DisplayName    string  `json:"displayName"`
	UnRead         int64   `json:"unread"`
	LatestMessage  Message `json:"latestMessage"`
}

type MessageHistoryRes struct {
	Messages   []Message `json:"messages"`
	NextCursor int64     `json:"nextCursor"`
	HasMore    bool      `json:"hasMore"`
}

type Danmaku struct {
	MessageId string `json:"messageId"`
	SenderId  string `json:"senderId"`
	Content   string `json:"content"`
	Seq       int64  `json:"seq"`
	TimeMs    int64  `json:"timeMs"`
	SendTime  int64  `json:"sendTime"`
}

type DanmakuRes struct {
	Items []Danmaku `json:"items"`
}

func toHistoryMessageRes(messages []messageapp.MessageAppeDTO, nextCursor int64, hashMore bool) (res MessageHistoryRes) {
	ms := make([]Message, 0, len(messages))

	for _, m := range messages {
		message := Message{
			MessageId:      m.MessageId,
			SenderId:       m.SendId,
			SenderUsername: m.SenderUsername,
			Seq:            m.Seq,
			ConvType:       m.ConvType,
			CType:          m.CType,
			Content:        m.Content,
			SendTime:       m.SendTime,
			VideoId:        m.VideoId,
			VideoTime:      m.VideoTime,
		}

		ms = append(ms, message)
	}

	res.Messages = ms
	res.HasMore = hashMore
	res.NextCursor = nextCursor

	return
}

func toDanmakuRes(items []messageapp.DanmakuDTO) (res DanmakuRes) {
	res.Items = make([]Danmaku, 0, len(items))
	for _, item := range items {
		res.Items = append(res.Items, Danmaku{
			MessageId: item.MessageId,
			SenderId:  item.SenderId,
			Content:   item.Content,
			Seq:       item.Seq,
			TimeMs:    item.TimeMs,
			SendTime:  item.SendTime,
		})
	}
	return res
}

func toOfflineMessageRes(messages []messageapp.MessageAppeDTO, unreadMap map[string]int64) (res []OfflineMessageRes) {
	for _, m := range messages {
		latestMessage := Message{
			MessageId:      m.MessageId,
			SenderId:       m.SendId,
			SenderUsername: m.SenderUsername,
			Seq:            m.Seq,
			ConvType:       m.ConvType,
			CType:          m.CType,
			Content:        m.Content,
			SendTime:       m.SendTime,
			VideoId:        m.VideoId,
			VideoTime:      m.VideoTime,
		}

		om := OfflineMessageRes{
			ConversationId: m.ConversationID,
			DisplayName:    m.DisplayName,
			UnRead:         unreadMap[m.ConversationID],
			LatestMessage:  latestMessage,
		}

		res = append(res, om)
	}

	return
}
