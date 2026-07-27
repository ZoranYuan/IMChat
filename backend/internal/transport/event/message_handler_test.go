package event

import (
	eventbus "IM_backend/internal/application/ports/eventbus"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type deliveryStub struct {
	eventType string
	userID    string
	payload   []byte
}

func (stub *deliveryStub) DeliverToUser(eventType, userID string, payload []byte) error {
	stub.eventType = eventType
	stub.userID = userID
	stub.payload = payload
	return nil
}

func TestMessageHandlerDecodesEnvelope(t *testing.T) {
	event := protocol.MessageEvent{MessageId: "m1", ConvType: protocol.PrivateChat, Seq: 2}
	eventPayload, _ := json.Marshal(event)
	payload, _ := json.Marshal(protocol.Envelope{From: "u1", To: "u2", Payload: eventPayload})
	stub := &deliveryStub{}
	handler := NewMessageHandler(stub, nil, nil, nil, MessageHandlerOptions{})

	err := handler.Handle(context.Background(), eventbus.IncomingEvent{
		Name:    protocol.EventTypeSendMessage,
		Key:     []byte("c1"),
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("处理消息失败：%v", err)
	}
	if stub.eventType != protocol.EventTypeSendMessage || stub.userID != "u2" {
		t.Fatalf("投递目标错误：event=%s user=%s", stub.eventType, stub.userID)
	}

	var got protocol.MessageEvent
	if err := json.Unmarshal(stub.payload, &got); err != nil {
		t.Fatalf("解析投递载荷失败：%v", err)
	}
	if got.MessageId != "m1" || got.Seq != 2 {
		t.Fatalf("投递载荷错误：%+v", got)
	}
}

func TestMessageHandlerMarksMalformedPayloadPermanent(t *testing.T) {
	handler := NewMessageHandler(&deliveryStub{}, nil, nil, nil, MessageHandlerOptions{})
	err := handler.Handle(context.Background(), eventbus.IncomingEvent{Payload: []byte("not-json")})

	var permanentError *eventbus.NonRetryableError
	if !errors.As(err, &permanentError) {
		t.Fatalf("非法消息应标记为永久错误：%v", err)
	}
}

type deliveryRecord struct {
	eventType string
	userID    string
	payload   []byte
}

type recordingDelivery struct {
	records []deliveryRecord
}

func (stub *recordingDelivery) DeliverToUser(eventType, userID string, payload []byte) error {
	stub.records = append(stub.records, deliveryRecord{
		eventType: eventType,
		userID:    userID,
		payload:   payload,
	})
	return nil
}

type roomRepositoryStub struct {
	room *roomentity.Room
	err  error
}

func (stub *roomRepositoryStub) Create(*roomentity.Room) error {
	return nil
}

func (stub *roomRepositoryStub) WithTx(any) roomrepo.RoomRepository {
	return stub
}

func (stub *roomRepositoryStub) FindActiveRoom(string, int) (*roomentity.Room, error) {
	if stub.err != nil {
		return nil, stub.err
	}
	return stub.room, nil
}

type roomUserRepositoryStub struct {
	members []string
	called  bool
}

func (stub *roomUserRepositoryStub) JoinRoom(user *roomentity.RoomUser) (*roomentity.RoomUser, error) {
	return user, nil
}

func (stub *roomUserRepositoryStub) WithTx(any) roomrepo.RoomUserRepository {
	return stub
}

func (stub *roomUserRepositoryStub) RejoinRoom(*roomentity.RoomUser) error {
	return nil
}

func (stub *roomUserRepositoryStub) GetRelationByIDs(string, string) (*roomentity.RoomUser, error) {
	return nil, nil
}

func (stub *roomUserRepositoryStub) ListActiveUserIDs(string) ([]string, error) {
	stub.called = true
	return stub.members, nil
}

func (stub *roomUserRepositoryStub) LeaveRoom(*roomentity.RoomUser, []int) error {
	return nil
}

type roomMemberCacheStub struct {
	members []string
	cached  bool
}

func (stub *roomMemberCacheStub) GetMember(context.Context, string, string) (*roomcache.MemberState, bool, error) {
	return nil, false, nil
}

func (stub *roomMemberCacheStub) SetMember(context.Context, string, string, *roomcache.MemberState) error {
	return nil
}

func (stub *roomMemberCacheStub) SetMemberNotFound(context.Context, string, string) error {
	return nil
}

func (stub *roomMemberCacheStub) DeleteMember(context.Context, string, string) error {
	return nil
}

func (stub *roomMemberCacheStub) GetMemberIDs(context.Context, string) ([]string, bool, error) {
	return stub.members, stub.cached, nil
}

func (stub *roomMemberCacheStub) SetMemberIDs(_ context.Context, _ string, userIDs []string) error {
	stub.members = userIDs
	stub.cached = true
	return nil
}

func (stub *roomMemberCacheStub) DeleteMemberIDs(context.Context, string) error {
	return nil
}

func TestMessageHandlerDeliversRoomMessageToMemberUsers(t *testing.T) {
	event := protocol.MessageEvent{MessageId: "m1", ConvType: protocol.RoomChat, Seq: 2}
	eventPayload, _ := json.Marshal(event)
	payload, _ := json.Marshal(protocol.Envelope{From: "u1", To: "room1", Payload: eventPayload})
	delivery := &recordingDelivery{}
	roomUsers := &roomUserRepositoryStub{members: []string{"u1", "u2", "u3"}}
	handler := NewMessageHandler(
		delivery,
		&roomRepositoryStub{room: &roomentity.Room{
			RoomId:      "room1",
			Status:      roomvo.Normal,
			MemberCount: 3,
		}},
		roomUsers,
		&roomMemberCacheStub{},
		MessageHandlerOptions{RoomRealtimeFanoutLimit: 500},
	)

	err := handler.Handle(context.Background(), eventbus.IncomingEvent{
		Name:    protocol.EventTypeSendMessage,
		Key:     []byte("room1"),
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("处理群消息失败：%v", err)
	}
	if len(delivery.records) != 2 {
		t.Fatalf("群消息应推给两个非发送者成员，实际=%d", len(delivery.records))
	}
	if delivery.records[0].userID != "u2" || delivery.records[1].userID != "u3" {
		t.Fatalf("群消息推送目标错误：%+v", delivery.records)
	}
}

func TestMessageHandlerDeliversNoticeForLargeRoom(t *testing.T) {
	event := protocol.MessageEvent{MessageId: "m1", ConversationId: "room1", ConvType: protocol.RoomChat, Seq: 2}
	eventPayload, _ := json.Marshal(event)
	payload, _ := json.Marshal(protocol.Envelope{From: "u1", To: "room1", Payload: eventPayload})
	delivery := &recordingDelivery{}
	roomUsers := &roomUserRepositoryStub{members: []string{"u1", "u2"}}
	handler := NewMessageHandler(
		delivery,
		&roomRepositoryStub{room: &roomentity.Room{
			RoomId:      "room1",
			Status:      roomvo.Normal,
			MemberCount: 501,
		}},
		roomUsers,
		&roomMemberCacheStub{},
		MessageHandlerOptions{RoomRealtimeFanoutLimit: 500},
	)

	err := handler.Handle(context.Background(), eventbus.IncomingEvent{
		Name:    protocol.EventTypeSendMessage,
		Key:     []byte("room1"),
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("处理大群消息失败：%v", err)
	}
	if len(delivery.records) != 1 {
		t.Fatalf("大群应向非发送者成员投递轻量提醒，实际=%d", len(delivery.records))
	}
	if delivery.records[0].eventType != protocol.EventRoomMessageNotice || delivery.records[0].userID != "u2" {
		t.Fatalf("大群轻量提醒目标错误：%+v", delivery.records[0])
	}
	var notice protocol.MessageNotifyEvent
	if err := json.Unmarshal(delivery.records[0].payload, &notice); err != nil {
		t.Fatalf("解析大群轻量提醒失败：%v", err)
	}
	if notice.ConversationId != "room1" || notice.MessageId != "m1" || notice.Seq != 2 {
		t.Fatalf("大群轻量提醒载荷错误：%+v", notice)
	}
	if !roomUsers.called {
		t.Fatal("大群当前阶段仍应查询成员列表，用于逐用户尝试 notice 投递")
	}
}
