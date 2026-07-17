package mq

import (
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type deliveryCall struct {
	eventType string
	userID    string
	payload   []byte
}

type fakeRealtimeDelivery struct {
	calls []deliveryCall
}

func (d *fakeRealtimeDelivery) DeliverToUser(eventType, userID string, payload []byte) error {
	d.calls = append(d.calls, deliveryCall{eventType: eventType, userID: userID, payload: payload})
	return nil
}

type fakeUserConversationRepository struct {
	updated *messageentity.UserConversation
	batch   []*messageentity.UserConversation
}

func (r *fakeUserConversationRepository) CreateUserConversation(context.Context, *messageentity.UserConversation) error {
	return nil
}
func (r *fakeUserConversationRepository) GetUsersByConversationID(context.Context, string) ([]string, error) {
	return nil, nil
}
func (r *fakeUserConversationRepository) BatchUpdateSyncSeq(_ context.Context, items []*messageentity.UserConversation) error {
	r.batch = items
	return nil
}
func (r *fakeUserConversationRepository) GetUserConversation(context.Context, string, string) (*messageentity.UserConversation, error) {
	return nil, nil
}
func (r *fakeUserConversationRepository) UpdateSyncSeq(_ context.Context, item *messageentity.UserConversation) error {
	r.updated = item
	return nil
}
func (r *fakeUserConversationRepository) UpdateReadSeq(context.Context, *messageentity.UserConversation) error {
	return nil
}
func (r *fakeUserConversationRepository) ListByUser(context.Context, string) ([]*messageentity.UserConversation, error) {
	return nil, nil
}
func (r *fakeUserConversationRepository) WithTx(any) messagerepo.UserConversationRepository {
	return r
}

func TestConsumerRouterDispatchesByTopic(t *testing.T) {
	called := false
	router := NewConsumerRouter(map[string]ConsumerHandler{
		"messages": ConsumerHandlerFunc(func(_ context.Context, message ConsumerMessage) error {
			called = message.Topic == "messages"
			return nil
		}),
	})

	if err := router.Handle(context.Background(), ConsumerMessage{Topic: "messages"}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !called {
		t.Fatal("registered handler was not called")
	}
	if err := router.Handle(context.Background(), ConsumerMessage{Topic: "unknown"}); !errors.Is(err, ErrHandlerNotFound) {
		t.Fatalf("unknown topic error = %v", err)
	}
}

func TestMessagePushHandlerPrivateMessage(t *testing.T) {
	delivery := &fakeRealtimeDelivery{}
	repository := &fakeUserConversationRepository{}
	handler := NewMessagePushHandler(delivery, nil, nil, repository, nil)
	event := protocol.MessageEvent{ConvType: protocol.PrivateChat, Seq: 12, Content: "hello"}

	if err := handler.Handle(context.Background(), ConsumerMessage{
		Topic: protocol.EventTypeMessage,
		Key:   []byte("conversation-1"),
		Value: envelopeJSON(t, protocol.Envelope{To: "user-2", Payload: jsonBytes(t, event)}),
	}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if len(delivery.calls) != 1 || delivery.calls[0].userID != "user-2" {
		t.Fatalf("unexpected delivery calls: %+v", delivery.calls)
	}
	if repository.updated == nil || repository.updated.ConversationId != "conversation-1" || repository.updated.LatestSyncSeq != 12 {
		t.Fatalf("unexpected sync update: %+v", repository.updated)
	}
}

func TestReadAckAndConversationSyncHandlers(t *testing.T) {
	delivery := &fakeRealtimeDelivery{}
	readHandler := NewReadAckHandler(delivery)
	readEvent := protocol.MessageReadAckEvent{SenderId: "sender-1", LastReadSeq: 7}
	if err := readHandler.Handle(context.Background(), ConsumerMessage{
		Value: envelopeJSON(t, protocol.Envelope{Payload: jsonBytes(t, readEvent)}),
	}); err != nil {
		t.Fatalf("read Handle() error = %v", err)
	}
	if len(delivery.calls) != 1 || delivery.calls[0].eventType != protocol.EventMessageReadNotify {
		t.Fatalf("unexpected read delivery: %+v", delivery.calls)
	}

	repository := &fakeUserConversationRepository{}
	syncHandler := NewConversationSyncHandler(repository)
	syncEvent := protocol.ConversationSyncSeqEvent{Items: []protocol.ConversationSyncSeqItem{
		{UserId: "user-1", ConversationId: "conversation-1", LatestSeq: 9},
		{UserId: "", ConversationId: "invalid", LatestSeq: 10},
	}}
	if err := syncHandler.Handle(context.Background(), ConsumerMessage{
		Value: envelopeJSON(t, protocol.Envelope{Payload: jsonBytes(t, syncEvent)}),
	}); err != nil {
		t.Fatalf("sync Handle() error = %v", err)
	}
	if len(repository.batch) != 1 || repository.batch[0].UserId != "user-1" || repository.batch[0].LatestSyncSeq != 9 {
		t.Fatalf("unexpected sync batch: %+v", repository.batch)
	}
}

func jsonBytes(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func envelopeJSON(t *testing.T, envelope protocol.Envelope) []byte {
	t.Helper()
	return jsonBytes(t, envelope)
}
