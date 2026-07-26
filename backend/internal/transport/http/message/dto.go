package message

import (
	messageapp "IM_backend/internal/application/message"
)

type MessageHistoryReq struct {
	ConversationId string `form:"conversationId" binding:"required"`
	ConvType       *int   `form:"convType"`
	Cursor         int64  `form:"cursor"`
	Limit          int    `form:"limit"`
}

type MessageSyncReq struct {
	ConversationId string `form:"conversationId" binding:"required"`
	AfterSeq       int64  `form:"afterSeq"`
	Limit          int    `form:"limit"`
}

type DanmakuReq struct {
	RoomId    string `form:"roomId" binding:"required"`
	VideoId   string `form:"videoId" binding:"required"`
	StartTime int64  `form:"startTime"`
	EndTime   int64  `form:"endTime"`
	Limit     int    `form:"limit"`
}

type RoomVideoHistoryReq struct {
	RoomId string `form:"roomId" binding:"required"`
	Limit  int    `form:"limit"`
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
	MediaURL       string `json:"mediaUrl,omitempty"`
	ThumbURL       string `json:"thumbUrl,omitempty"`
	FileId         string `json:"fileId,omitempty"`
	ThumbFileId    string `json:"thumbFileId,omitempty"`
	FileName       string `json:"fileName,omitempty"`
	FileSize       int64  `json:"fileSize,omitempty"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	DurationMs     *int64 `json:"durationMs,omitempty"`
	StickerId      string `json:"stickerId,omitempty"`
	PackId         string `json:"packId,omitempty"`
}

type MessageHistoryRes struct {
	Messages   []Message `json:"messages"`
	NextCursor int64     `json:"nextCursor"`
	HasMore    bool      `json:"hasMore"`
}

type MessageSyncRes struct {
	Messages []Message `json:"messages"`
	NextSeq  int64     `json:"nextSeq"`
	HasMore  bool      `json:"hasMore"`
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

type RoomVideoHistoryItem struct {
	VideoId        string `json:"videoId"`
	FileName       string `json:"fileName"`
	LatestSendTime int64  `json:"latestSendTime"`
	VideoTime      *int64 `json:"videoTime,omitempty"`
	MessageCount   int64  `json:"messageCount"`
}

type RoomVideoHistoryRes struct {
	Items []RoomVideoHistoryItem `json:"items"`
}

func toHistoryMessageRes(messages []messageapp.MessageAppeDTO, nextCursor int64, hashMore bool) (res MessageHistoryRes) {
	ms := toMessagesRes(messages)

	res.Messages = ms
	res.HasMore = hashMore
	res.NextCursor = nextCursor

	return
}

func toSyncMessageRes(messages []messageapp.MessageAppeDTO, nextSeq int64, hashMore bool) (res MessageSyncRes) {
	res.Messages = toMessagesRes(messages)
	res.HasMore = hashMore
	res.NextSeq = nextSeq
	return
}

func toMessagesRes(messages []messageapp.MessageAppeDTO) []Message {
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
			MediaURL:       m.MediaURL,
			ThumbURL:       m.ThumbURL,
			FileId:         m.FileId,
			ThumbFileId:    m.ThumbFileId,
			FileName:       m.FileName,
			FileSize:       m.FileSize,
			Width:          m.Width,
			Height:         m.Height,
			DurationMs:     m.DurationMs,
			StickerId:      m.StickerId,
			PackId:         m.PackId,
		}

		ms = append(ms, message)
	}

	return ms
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

func toRoomVideoHistoryRes(items []messageapp.RoomVideoHistoryDTO) (res RoomVideoHistoryRes) {
	res.Items = make([]RoomVideoHistoryItem, 0, len(items))
	for _, item := range items {
		res.Items = append(res.Items, RoomVideoHistoryItem{
			VideoId:        item.VideoId,
			FileName:       item.FileName,
			LatestSendTime: item.LatestSendTime,
			VideoTime:      item.VideoTime,
			MessageCount:   item.MessageCount,
		})
	}
	return res
}
