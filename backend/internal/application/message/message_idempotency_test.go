package message

import (
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	"testing"
)

func TestSameClientMessage(t *testing.T) {
	videoTime := int64(1200)
	existing := &messageentity.Message{
		ConversationId: "u2_u1",
		Type:           messagevo.Text,
		Content:        "hello",
		VideoTime:      &videoTime,
	}

	dto := MessageAppeDTO{
		SendId:    "u1",
		RecvId:    "u2",
		ConvType:  1,
		CType:     int(messagevo.Text),
		Content:   "hello",
		VideoTime: &videoTime,
	}
	if !sameClientMessage(dto, existing) {
		t.Fatal("相同客户端消息应被识别为重复请求")
	}

	dto.Content = "changed"
	if sameClientMessage(dto, existing) {
		t.Fatal("相同 clientMsgId 但内容不同不应被识别为重复请求")
	}
}
