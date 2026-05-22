package message

import messageentity "IM_backend/internal/domain/message/entity"

type MessageAppeDTO struct {
	ClientMsgId    string `json:"clientMsgId"`      // 客户端消息ID（用于ACK去重）
	SendId         string `json:"sendId"`           // 发送者ID
	SenderUsername string `json:"senderUsername"`   // 发送者用户名
	RecvId         string `json:"recvId,omitempty"` // 接收者ID（单聊是用户ID，群聊是群ID）
	ConversationID string `json:"conversation"`
	DisplayName    string `json:"displayName"`
	MessageId      string `json:"messageId"`
	Status         string `json:"status"`
	Seq            int64  `json:"seq"`
	ConvType       int    `json:"convType"` // 会话类型（单聊/群聊）
	CType          int    `json:"cType"`    // 消息类型（文本/图片/视频等）
	Content        string `json:"content"`  // 消息内容
	SendTime       int64  `json:"sendTime"`
	VideoId        string `json:"videoId"`
	VideoTime      *int64 `json:"videoTime"` // 视频时长（毫秒，可选）
}

type DanmakuDTO struct {
	MessageId string `json:"messageId"`
	SenderId  string `json:"senderId"`
	Content   string `json:"content"`
	Seq       int64  `json:"seq"`
	TimeMs    int64  `json:"timeMs"`
	SendTime  int64  `json:"sendTime"`
}

func toMessagesAppDTO(ms []*messageentity.Message) []MessageAppeDTO {
	if len(ms) == 0 {
		return nil
	}

	res := make([]MessageAppeDTO, 0, len(ms))
	for _, m := range ms {
		if m == nil {
			continue
		}

		res = append(res, MessageAppeDTO{
			SendId:         m.SendId,
			ConversationID: m.ConversationId,
			MessageId:      m.MessageId,
			Seq:            m.Seq,
			CType:          int(m.Type),
			Content:        m.Content,
			SendTime:       m.SendTime,
			VideoId:        m.VideoId,
			VideoTime:      m.VideoTime,
		})
	}

	return res
}
