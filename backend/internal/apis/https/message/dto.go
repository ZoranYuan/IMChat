package https_message

import application_message "IM_backend/internal/applications/message"

type MessageHistoryReq struct {
	ConversationId string `form:"conversationId" binding:"required"`
	Cursor         int64  `form:"cursor"`
	Limit          int    `form:"limit"`
}

type MessageHistoryItemRes struct {
	MessageId      string `json:"messageId"`
	ConversationId string `json:"conversationId"`
	SenderId       string `json:"senderId"`
	Seq            int64  `json:"seq"`
	ConvType       int    `json:"convType"` // 单聊/群聊
	CType          int    `json:"cType"`    // 消息类型（文本/图片等）
	Content        string `json:"content"`
	SendTime       int64  `json:"sendTime"`
}

func toMessageHistoryRes(list []application_message.MessageAppeDTO) []MessageHistoryItemRes {
	res := make([]MessageHistoryItemRes, 0, len(list))

	for _, i := range list {
		res = append(res, MessageHistoryItemRes{
			MessageId:      i.MessageId,
			ConversationId: i.ConsersationId,
			SenderId:       i.SendId,
			Seq:            i.Seq,
			ConvType:       i.ConvType,
			CType:          i.CType,
			Content:        i.Content,
			SendTime:       i.SendTime,
		})
	}

	return res
}

type MessageHistoryRes struct {
	MessageHistoryList []MessageHistoryItemRes `json:"messageHistoryList"`
	NextCursor         int64                   `json:"nextCursor"`
	HasMore            bool                    `json:"hasMore"`
}
