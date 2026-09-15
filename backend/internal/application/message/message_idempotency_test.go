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

	dto := SendMessageDTO{
		SenderID:         "u1",
		ReceiverID:       "u2",
		ConversationType: 1,
		Type:             int(messagevo.Text),
		Content:          "hello",
		VideoTime:        &videoTime,
	}
	if !sameClientMessage(dto, existing, "") {
		t.Fatal("相同客户端消息应被识别为重复请求")
	}

	dto.Content = "changed"
	if sameClientMessage(dto, existing, "") {
		t.Fatal("相同 clientMsgId 但内容不同不应被识别为重复请求")
	}
}

func TestMessageRequestHashIncludesFileIdentity(t *testing.T) {
	base := SendMessageDTO{
		ConversationID: "u1_u2",
		Type:           int(messagevo.Image),
		Content:        "photo.png",
		FileID:         "file-1",
		Width:          100,
		Height:         100,
	}

	first, err := buildMessageRequestHash(base)
	if err != nil {
		t.Fatalf("生成第一条消息摘要失败：%v", err)
	}

	base.FileID = "file-2"
	second, err := buildMessageRequestHash(base)
	if err != nil {
		t.Fatalf("生成第二条消息摘要失败：%v", err)
	}
	if first == second {
		t.Fatal("不同 fileId 不应生成相同消息摘要")
	}

	base.FileID = "file-1"
	base.Width = 300
	base.Height = 200
	base.DurationMs = func() *int64 {
		value := int64(9000)
		return &value
	}()
	third, err := buildMessageRequestHash(base)
	if err != nil {
		t.Fatalf("生成带旧版媒体元数据的消息摘要失败：%v", err)
	}
	if first != third {
		t.Fatal("媒体消息的幂等摘要不应依赖客户端宽高和时长")
	}
}
