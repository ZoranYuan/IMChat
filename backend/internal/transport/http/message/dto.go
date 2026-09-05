package message

import (
	messageapp "IM_backend/internal/application/message"
)

type MessageHistoryRequest struct {
	ConversationID string `form:"conversationId" binding:"required"`
	Cursor         int64  `form:"cursor"`
	Limit          int    `form:"limit"`
}

type MessageSyncRequest struct {
	ConversationID string `form:"conversationId" binding:"required"`
	AfterSeq       int64  `form:"afterSeq"`
}

type MessageSeqsRequest struct {
	ConversationID string `form:"conversationId" binding:"required"`
	Seqs           string `form:"seqs" binding:"required"`
}

type DanmakuRequest struct {
	RoomID    string `form:"roomId" binding:"required"`
	VideoID   string `form:"videoId" binding:"required"`
	StartTime int64  `form:"startTime"`
	EndTime   int64  `form:"endTime"`
	Limit     int    `form:"limit"`
}

type RoomVideoHistoryRequest struct {
	RoomID string `form:"roomId" binding:"required"`
	Limit  int    `form:"limit"`
}

type MessageResponse struct {
	MessageID       string  `json:"messageId"`
	ConversationID  string  `json:"conversationId"`
	SenderID        string  `json:"senderId"`
	ClientMessageID *string `json:"clientMsgId,omitempty"`
	Seq             int64   `json:"seq"`
	Type            int     `json:"cType"`
	Content         string  `json:"content"`
	VideoID         string  `json:"videoId,omitempty"`
	VideoTime       *int64  `json:"videoTime,omitempty"`
	Status          int8    `json:"status"`
	SendTime        int64   `json:"sendTime"`
	AttachmentID    string  `json:"attachmentId,omitempty"`
}

type MessageHistoryResponse struct {
	Messages   []MessageResponse `json:"messages"`
	NextCursor int64             `json:"nextCursor"`
	HasMore    bool              `json:"hasMore"`
}

type MessageSyncResponse struct {
	Messages []MessageResponse `json:"messages"`
}

type MessageSeqsResponse struct {
	Messages []MessageResponse `json:"messages"`
}

type DanmakuResponse struct {
	MessageID string `json:"messageId"`
	SenderID  string `json:"senderId"`
	Content   string `json:"content"`
	Seq       int64  `json:"seq"`
	TimeMs    int64  `json:"timeMs"`
	SendTime  int64  `json:"sendTime"`
}

type DanmakuListResponse struct {
	Items []DanmakuResponse `json:"items"`
}

type RoomVideoHistoryResponseItem struct {
	VideoID        string `json:"videoId"`
	FileName       string `json:"fileName"`
	LatestSendTime int64  `json:"latestSendTime"`
	VideoTime      *int64 `json:"videoTime,omitempty"`
	MessageCount   int64  `json:"messageCount"`
}

type RoomVideoHistoryResponse struct {
	Items []RoomVideoHistoryResponseItem `json:"items"`
}

func toHistoryMessageResponse(messages []messageapp.MessageDTO, nextCursor int64, hasMore bool) (res MessageHistoryResponse) {
	ms := toMessageResponses(messages)

	res.Messages = ms
	res.HasMore = hasMore
	res.NextCursor = nextCursor

	return
}

func toSyncMessageResponse(messages []messageapp.MessageDTO) (res MessageSyncResponse) {
	res.Messages = toMessageResponses(messages)
	return
}

func toMessageSeqsResponse(messages []messageapp.MessageDTO) (res MessageSeqsResponse) {
	res.Messages = toMessageResponses(messages)
	return
}

func toMessageResponses(messages []messageapp.MessageDTO) []MessageResponse {
	ms := make([]MessageResponse, 0, len(messages))

	for _, m := range messages {
		message := MessageResponse{
			MessageID:       m.MessageID,
			ConversationID:  m.ConversationID,
			SenderID:        m.SenderID,
			ClientMessageID: m.ClientMessageID,
			Seq:             m.Seq,
			Type:            m.Type,
			Content:         m.Content,
			VideoID:         m.VideoID,
			VideoTime:       m.VideoTime,
			Status:          m.Status,
			SendTime:        m.SendTime,
			AttachmentID:    m.AttachmentID,
		}

		ms = append(ms, message)
	}

	return ms
}

func toDanmakuResponse(items []messageapp.DanmakuDTO) (res DanmakuListResponse) {
	res.Items = make([]DanmakuResponse, 0, len(items))
	for _, item := range items {
		res.Items = append(res.Items, DanmakuResponse{
			MessageID: item.MessageID,
			SenderID:  item.SenderID,
			Content:   item.Content,
			Seq:       item.Seq,
			TimeMs:    item.TimeMs,
			SendTime:  item.SendTime,
		})
	}
	return res
}

func toRoomVideoHistoryResponse(items []messageapp.RoomVideoHistoryDTO) (res RoomVideoHistoryResponse) {
	res.Items = make([]RoomVideoHistoryResponseItem, 0, len(items))
	for _, item := range items {
		res.Items = append(res.Items, RoomVideoHistoryResponseItem{
			VideoID:        item.VideoID,
			FileName:       item.FileName,
			LatestSendTime: item.LatestSendTime,
			VideoTime:      item.VideoTime,
			MessageCount:   item.MessageCount,
		})
	}
	return res
}
