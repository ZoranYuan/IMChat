package message

import (
	"IM_backend/configs"
	conversationrepo "IM_backend/internal/application/ports/persistence/repository/conversation"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	conversationvo "IM_backend/internal/domain/conversation/value_object"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	roomcacheimpl "IM_backend/internal/infrastructure/persistence/redis/cache/room"
	"IM_backend/internal/shared/protocol"
	"context"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type syncConversationRepositoryStub struct {
	conversationrepo.ConversationRepository
	conversation *conversationentity.Conversation
}

func (stub *syncConversationRepositoryStub) GetByID(context.Context, string) (*conversationentity.Conversation, error) {
	return stub.conversation, nil
}

type syncRoomUserRepositoryStub struct {
	roomrepo.RoomUserRepository
}

func (stub *syncRoomUserRepositoryStub) GetRelationByIDs(string, string) (*roomentity.RoomUser, error) {
	return &roomentity.RoomUser{Status: roomvo.Activate}, nil
}

type syncMessageRepositoryStub struct {
	messagerepo.MessageRepository
	messages []*messageentity.Message
	calls    int
}

func (stub *syncMessageRepositoryStub) ListAfterSeq(context.Context, string, int64) ([]*messageentity.Message, error) {
	stub.calls++
	return stub.messages, nil
}

func TestSyncMessagesFallsBackToDatabaseAndWarmsGap(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})

	roomCache := roomcacheimpl.NewRoomCache(client, configs.MessageConfig{
		RoomActivityWindowSeconds:  30,
		RoomActivityBucketSeconds:  5,
		RoomActivityKeyTTLSeconds:  60,
		RoomActivityWarnMessages:   30,
		RoomActivityActiveMessages: 90,
	})
	ctx := context.Background()
	for _, seq := range []int64{100, 102} {
		if err := roomCache.AppendRecentMessageSeq(ctx, "room-1", protocolMessageEvent(seq)); err != nil {
			t.Fatal(err)
		}
	}

	messageRepository := &syncMessageRepositoryStub{
		messages: []*messageentity.Message{
			syncMessageEntity(100),
			syncMessageEntity(101),
			syncMessageEntity(102),
			syncMessageEntity(103),
		},
	}
	app := &MessageApplication{
		conversationRepository: &syncConversationRepositoryStub{
			conversation: &conversationentity.Conversation{
				ConversationId: "room-1",
				RoomId:         "room-1",
				Convtype:       conversationvo.RoomChat,
				LatestSeq:      103,
			},
		},
		roomUserRepository: &syncRoomUserRepositoryStub{},
		messageRepository:  messageRepository,
		roomCache:          roomCache,
	}

	messages, err := app.SyncMessages(ctx, "room-1", "user-1", 99)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 4 {
		t.Fatalf("数据库回源结果错误：messages=%+v", messages)
	}
	if messageRepository.calls != 1 {
		t.Fatalf("缓存缺口应只触发一次数据库查询，实际=%d", messageRepository.calls)
	}

	page, err := roomCache.ListMessageAfterSeq(ctx, "room-1", 99, 103)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Covered || len(page.Events) != 4 {
		t.Fatalf("数据库结果没有正确回填缓存：page=%+v", page)
	}
}

func syncMessageEntity(seq int64) *messageentity.Message {
	return &messageentity.Message{
		MessageId:      "message-" + strconv.FormatInt(seq, 10),
		ConversationId: "room-1",
		SenderId:       "user-1",
		Seq:            seq,
		Type:           messagevo.Text,
		Content:        "message",
		SendTime:       seq,
	}
}

func protocolMessageEvent(seq int64) protocol.MessageEvent {
	return protocol.MessageEvent{
		MessageId:      "message-" + strconv.FormatInt(seq, 10),
		ConversationId: "room-1",
		SenderId:       "user-1",
		Seq:            seq,
		ConvType:       protocol.RoomChat,
		CType:          int(messagevo.Text),
		Content:        "message",
	}
}
