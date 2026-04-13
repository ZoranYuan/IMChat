package application_message

import message_entity "IM_backend/internal/domain/message/entity"

type MessageAppeDTO struct {
	ClientMsgId    string `json:"clientMsgId"`      // 客户端消息ID（用于ACK去重）
	SendId         string `json:"sendId"`           // 发送者ID
	RecvId         string `json:"recvId,omitempty"` // 接收者ID（单聊是用户ID，群聊是群ID）
	ConsersationId string `json:"conversation"`
	MessageId      string `json:"messageId"`
	Seq            int64  `json:"seq"`
	ConvType       int    `json:"convType"` // 会话类型（单聊/群聊）
	CType          int    `json:"cType"`    // 消息类型（文本/图片/视频等）
	Content        string `json:"content"`  // 消息内容
	SendTime       int64  `json:"sendTime"`
	VideoTime      *int64 `json:"videoTime"` // 视频时长（毫秒，可选）
}

func toMessagesAppDTO(ms []*message_entity.Message) []MessageAppeDTO {
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
			ConsersationId: m.ConversationId,
			MessageId:      m.MessageId,
			Seq:            m.Seq,
			CType:          int(m.Type),
			Content:        m.Content,
			SendTime:       m.SendTime,
			VideoTime:      m.VideoTime,
		})
	}

	return res
}
