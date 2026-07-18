package message

import (
	serviceport "IM_backend/internal/application/ports/service"
	"IM_backend/internal/shared/protocol"
	"context"
	"errors"
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

type syncRecorder struct {
	steps *[]string
	err   error
}

func (recorder *syncRecorder) SyncLatestSequences(_ context.Context, _ []serviceport.ConversationSyncItem) error {
	*recorder.steps = append(*recorder.steps, "同步序列")
	return recorder.err
}

func TestDeliveryApplicationPrivateMessage(t *testing.T) {
	steps := make([]string, 0, 2)
	application := NewDeliveryApplication(
		&deliveryRecorder{steps: &steps},
		nil,
		nil,
		&syncRecorder{steps: &steps},
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

	want := []string{"同步序列", "投递:u2"}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("执行顺序错误：got=%v want=%v", steps, want)
	}
}

func TestDeliveryApplicationStopsWhenSyncFails(t *testing.T) {
	steps := make([]string, 0, 1)
	syncErr := errors.New("同步失败")
	application := NewDeliveryApplication(
		&deliveryRecorder{steps: &steps},
		nil,
		nil,
		&syncRecorder{steps: &steps, err: syncErr},
		nil,
	)

	err := application.DeliverMessage(context.Background(), DeliveryCommand{
		ConversationId: "c1",
		ToUserId:       "u2",
		Message: protocol.MessageEvent{
			ConvType: protocol.PrivateChat,
		},
	})
	if !errors.Is(err, syncErr) {
		t.Fatalf("期望同步错误，实际为：%v", err)
	}
	if !reflect.DeepEqual(steps, []string{"同步序列"}) {
		t.Fatalf("同步失败后不应投递：%v", steps)
	}
}
