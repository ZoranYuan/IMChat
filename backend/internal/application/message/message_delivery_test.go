package message

import (
	"IM_backend/configs"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"testing"
)

type deliveryRecord struct {
	eventType string
	userID    string
	payload   []byte
}

type roomDeliveryRecord struct {
	eventType     string
	roomID        string
	payload       []byte
	excludeUserID string
}

type recordingRealtimeDelivery struct {
	records     []deliveryRecord
	roomRecords []roomDeliveryRecord
}

func (stub *recordingRealtimeDelivery) DeliverToUser(eventType, userID string, payload []byte) error {
	stub.records = append(stub.records, deliveryRecord{
		eventType: eventType,
		userID:    userID,
		payload:   payload,
	})
	return nil
}

func (stub *recordingRealtimeDelivery) DeliverToOnlineRoomMembers(eventType, roomID string, payload []byte, excludeUserID string) error {
	stub.roomRecords = append(stub.roomRecords, roomDeliveryRecord{
		eventType: eventType, roomID: roomID, payload: payload, excludeUserID: excludeUserID,
	})
	return nil
}

type deliveryRoomRepositoryStub struct {
	room *roomentity.Room
	err  error
}

func (stub *deliveryRoomRepositoryStub) Create(*roomentity.Room) error {
	return nil
}

func (stub *deliveryRoomRepositoryStub) WithTx(any) roomrepo.RoomRepository {
	return stub
}

func (stub *deliveryRoomRepositoryStub) FindActiveRoom(string, int) (*roomentity.Room, error) {
	if stub.err != nil {
		return nil, stub.err
	}
	return stub.room, nil
}

func (stub *deliveryRoomRepositoryStub) IncrementMemberCount(context.Context, string, int) (bool, error) {
	return true, nil
}

func (stub *deliveryRoomRepositoryStub) DecrementMemberCount(context.Context, string, int) (bool, error) {
	return true, nil
}

type deliveryRoomUserRepositoryStub struct {
	members []string
	called  bool
}

func (stub *deliveryRoomUserRepositoryStub) JoinRoom(user *roomentity.RoomUser) (*roomentity.RoomUser, error) {
	return user, nil
}

func (stub *deliveryRoomUserRepositoryStub) WithTx(any) roomrepo.RoomUserRepository {
	return stub
}

func (stub *deliveryRoomUserRepositoryStub) RejoinRoom(*roomentity.RoomUser) error {
	return nil
}

func (stub *deliveryRoomUserRepositoryStub) GetRelationByIDs(string, string) (*roomentity.RoomUser, error) {
	return nil, nil
}

func (stub *deliveryRoomUserRepositoryStub) ListActiveUserIDs(string) ([]string, error) {
	stub.called = true
	return stub.members, nil
}

func (stub *deliveryRoomUserRepositoryStub) ListActiveRoomIDs(string) ([]string, error) {
	return nil, nil
}

func (stub *deliveryRoomUserRepositoryStub) LeaveRoom(*roomentity.RoomUser, []int) error {
	return nil
}

type deliveryRoomMemberCacheStub struct {
	members []string
	cached  bool
}

func testMessageConfig() configs.MessageConfig {
	return configs.MessageConfig{
		RoomRealtimeFanoutLimit:           500,
		LargeRoomNoticeLingerMilliseconds: 200,
		LargeRoomNoticeShardCount:         16,
		LargeRoomNoticeMaxPending:         100000,
	}
}

func (stub *deliveryRoomMemberCacheStub) GetMember(context.Context, string, string) (*roomcache.MemberState, bool, error) {
	return nil, false, nil
}

func (stub *deliveryRoomMemberCacheStub) SetMember(context.Context, string, string, *roomcache.MemberState) error {
	return nil
}

func (stub *deliveryRoomMemberCacheStub) SetMemberIfVersionGreater(context.Context, string, string, *roomcache.MemberState) (bool, error) {
	return true, nil
}

func (stub *deliveryRoomMemberCacheStub) SetMemberNotFound(context.Context, string, string) error {
	return nil
}

func (stub *deliveryRoomMemberCacheStub) DeleteMember(context.Context, string, string) error {
	return nil
}

func (stub *deliveryRoomMemberCacheStub) GetMemberIDs(context.Context, string) ([]string, bool, error) {
	return stub.members, stub.cached, nil
}

func (stub *deliveryRoomMemberCacheStub) SetMemberIDs(_ context.Context, _ string, userIDs []string) error {
	stub.members = userIDs
	stub.cached = true
	return nil
}

func (stub *deliveryRoomMemberCacheStub) DeleteMemberIDs(context.Context, string) error {
	return nil
}

func TestDeliveryDeliversPrivateMessageToReceiver(t *testing.T) {
	event := protocol.MessageEvent{MessageId: "m1", ConvType: protocol.PrivateChat, Seq: 2}
	eventPayload, _ := json.Marshal(event)
	envelope := protocol.Envelope{From: "u1", To: "u2", Payload: eventPayload}
	realtime := &recordingRealtimeDelivery{}
	delivery := NewMessageDelivery(realtime, nil, nil, nil, nil, testMessageConfig())
	defer delivery.Close(context.Background())

	err := delivery.Deliver(context.Background(), protocol.EventTypeSendMessage, "c1", envelope, event)
	if err != nil {
		t.Fatalf("投递私聊消息失败：%v", err)
	}
	if len(realtime.records) != 1 {
		t.Fatalf("私聊应投递 1 条，实际=%d", len(realtime.records))
	}
	if realtime.records[0].eventType != protocol.EventTypeSendMessage || realtime.records[0].userID != "u2" {
		t.Fatalf("私聊投递目标错误：%+v", realtime.records[0])
	}
}

func TestDeliveryDeliversSmallRoomMessageToMemberUsers(t *testing.T) {
	event := protocol.MessageEvent{MessageId: "m1", ConvType: protocol.RoomChat, Seq: 2}
	eventPayload, _ := json.Marshal(event)
	envelope := protocol.Envelope{From: "u1", To: "room1", Payload: eventPayload}
	realtime := &recordingRealtimeDelivery{}
	roomUsers := &deliveryRoomUserRepositoryStub{members: []string{"u1", "u2", "u3"}}
	delivery := NewMessageDelivery(
		realtime,
		&deliveryRoomRepositoryStub{room: &roomentity.Room{
			RoomId:      "room1",
			Status:      roomvo.Normal,
			MemberCount: 3,
		}},
		nil,
		roomUsers,
		&deliveryRoomMemberCacheStub{},
		testMessageConfig(),
	)
	defer delivery.Close(context.Background())

	err := delivery.Deliver(context.Background(), protocol.EventTypeSendMessage, "room1", envelope, event)
	if err != nil {
		t.Fatalf("投递小群消息失败：%v", err)
	}
	if len(realtime.records) != 2 {
		t.Fatalf("小群消息应推给两个非发送者成员，实际=%d", len(realtime.records))
	}
	if realtime.records[0].eventType != protocol.EventTypeSendMessage || realtime.records[0].userID != "u2" {
		t.Fatalf("小群第一个投递目标错误：%+v", realtime.records[0])
	}
	if realtime.records[1].eventType != protocol.EventTypeSendMessage || realtime.records[1].userID != "u3" {
		t.Fatalf("小群第二个投递目标错误：%+v", realtime.records[1])
	}
}

func TestDeliveryDeliversNoticeForLargeRoom(t *testing.T) {
	event := protocol.MessageEvent{MessageId: "m1", ConversationId: "room1", ConvType: protocol.RoomChat, Seq: 2}
	eventPayload, _ := json.Marshal(event)
	envelope := protocol.Envelope{From: "u1", To: "room1", Payload: eventPayload}
	realtime := &recordingRealtimeDelivery{}
	roomUsers := &deliveryRoomUserRepositoryStub{members: []string{"u1", "u2"}}
	delivery := NewMessageDelivery(
		realtime,
		&deliveryRoomRepositoryStub{room: &roomentity.Room{
			RoomId:      "room1",
			Status:      roomvo.Normal,
			MemberCount: 501,
		}},
		nil,
		roomUsers,
		&deliveryRoomMemberCacheStub{},
		testMessageConfig(),
	)
	defer delivery.Close(context.Background())

	err := delivery.Deliver(context.Background(), protocol.EventTypeSendMessage, "room1", envelope, event)
	if err != nil {
		t.Fatalf("投递大群消息失败：%v", err)
	}
	delivery.FlushLargeRoomNotices(context.Background())
	if len(realtime.records) != 0 {
		t.Fatalf("大群不应逐用户投递，实际=%d", len(realtime.records))
	}
	if len(realtime.roomRecords) != 1 {
		t.Fatalf("大群应投递 1 条在线成员轻量提醒，实际=%d", len(realtime.roomRecords))
	}
	if realtime.roomRecords[0].eventType != protocol.EventRoomMessageNotice || realtime.roomRecords[0].roomID != "room1" || realtime.roomRecords[0].excludeUserID != "u1" {
		t.Fatalf("大群轻量提醒目标错误：%+v", realtime.roomRecords[0])
	}
	var notice protocol.MessageNotifyEvent
	if err := json.Unmarshal(realtime.roomRecords[0].payload, &notice); err != nil {
		t.Fatalf("解析大群轻量提醒失败：%v", err)
	}
	if notice.ConversationId != "room1" || notice.MessageId != "m1" || notice.Seq != 2 {
		t.Fatalf("大群轻量提醒载荷错误：%+v", notice)
	}
	if roomUsers.called {
		t.Fatal("大群读扩散不应查询全量成员列表")
	}
}

func TestDeliveryCoalescesLargeRoomNoticeToLatestSeq(t *testing.T) {
	realtime := &recordingRealtimeDelivery{}
	roomUsers := &deliveryRoomUserRepositoryStub{members: []string{"u1", "u2"}}
	delivery := NewMessageDelivery(
		realtime,
		&deliveryRoomRepositoryStub{room: &roomentity.Room{
			RoomId:      "room1",
			Status:      roomvo.Normal,
			MemberCount: 501,
		}},
		nil,
		roomUsers,
		&deliveryRoomMemberCacheStub{},
		func() configs.MessageConfig {
			config := testMessageConfig()
			config.LargeRoomNoticeLingerMilliseconds = 1000
			return config
		}(),
	)
	defer delivery.Close(context.Background())

	for _, seq := range []int64{2, 3} {
		event := protocol.MessageEvent{MessageId: "m", ConversationId: "room1", ConvType: protocol.RoomChat, Seq: seq}
		eventPayload, _ := json.Marshal(event)
		envelope := protocol.Envelope{From: "u1", To: "room1", Payload: eventPayload}
		if err := delivery.Deliver(context.Background(), protocol.EventTypeSendMessage, "room1", envelope, event); err != nil {
			t.Fatalf("投递大群消息失败：%v", err)
		}
	}

	delivery.FlushLargeRoomNotices(context.Background())
	if len(realtime.roomRecords) != 1 {
		t.Fatalf("同一房间的轻量提醒应合并为 1 条，实际=%d", len(realtime.roomRecords))
	}
	var notice protocol.MessageNotifyEvent
	if err := json.Unmarshal(realtime.roomRecords[0].payload, &notice); err != nil {
		t.Fatalf("解析大群轻量提醒失败：%v", err)
	}
	if notice.Seq != 3 {
		t.Fatalf("合并后应保留最新 seq=3，实际=%d", notice.Seq)
	}
}
