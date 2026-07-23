package message

import (
	"IM_backend/internal/shared/protocol"
	"context"
	"reflect"
	"testing"
)

type deliveryRecorder struct {
	steps *[]string
}

func (recorder *deliveryRecorder) DeliverToUser(_ string, userID string, _ []byte) error {
	*recorder.steps = append(*recorder.steps, "投递:"+userID)
	return nil
}

func TestDeliveryApplicationPrivateMessage(t *testing.T) {
	steps := make([]string, 0, 1)
	application := NewDeliveryApplication(
		&deliveryRecorder{steps: &steps},
		nil,
		nil,
		nil,
	)

	err := application.DeliverMessage(context.Background(), DeliveryCommand{
		EventType:      protocol.EventTypeSendMessage,
		ConversationId: "c1",
		FromUserId:     "u1",
		ToUserId:       "u2",
		Payload:        []byte("payload"),
		Message: protocol.MessageEvent{
			ConvType: protocol.PrivateChat,
			Seq:      3,
		},
	})
	if err != nil {
		t.Fatalf("投递私聊消息失败：%v", err)
	}

	want := []string{"投递:u2"}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("执行顺序错误：got=%v want=%v", steps, want)
	}
}
