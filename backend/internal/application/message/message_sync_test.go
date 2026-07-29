package message

import (
	"IM_backend/configs"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	usercache "IM_backend/internal/application/ports/persistence/cache/user"
	conversationrepo "IM_backend/internal/application/ports/persistence/repository/conversation"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	conversationvo "IM_backend/internal/domain/conversation/value_object"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	userentity "IM_backend/internal/domain/user/entity"
	context "context"
	"errors"
	"testing"
	"time"
)

type syncMessageRepositoryStub struct {
	listAfterSeqConversationID string
	listAfterSeqAfterSeq       int64
	listAfterSeqLimit          int
	listAfterSeqResult         []*messageentity.Message
}

func (stub *syncMessageRepositoryStub) CreateNewMessages(context.Context, []*messageentity.Message) error {
	return nil
}

func (stub *syncMessageRepositoryStub) CreateNewMessage(context.Context, *messageentity.Message) error {
	return nil
}

func (stub *syncMessageRepositoryStub) FindByClientMsgID(context.Context, string, string) (*messageentity.Message, error) {
	return nil, nil
}

func (stub *syncMessageRepositoryStub) GetHistoryMessage(context.Context, string, int64, int) ([]*messageentity.Message, error) {
	return nil, nil
}

func (stub *syncMessageRepositoryStub) ListAfterSeq(_ context.Context, conversationId string, afterSeq int64, limit int) ([]*messageentity.Message, error) {
	stub.listAfterSeqConversationID = conversationId
	stub.listAfterSeqAfterSeq = afterSeq
	stub.listAfterSeqLimit = limit
	return stub.listAfterSeqResult, nil
}

func (stub *syncMessageRepositoryStub) GetMessagesBySendTime(context.Context, string, int64, int64, int) ([]*messageentity.Message, error) {
	return nil, nil
}

func (stub *syncMessageRepositoryStub) GetDanmakuByRoomVideo(context.Context, string, string, int64, int64, int) ([]*messageentity.Message, error) {
	return nil, nil
}

func (stub *syncMessageRepositoryStub) GetRoomVideoHistory(context.Context, string, int) ([]*messageentity.Message, error) {
	return nil, nil
}

func (stub *syncMessageRepositoryStub) CountRoomVideoMessages(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (stub *syncMessageRepositoryStub) GetLatestMessagesByConversationIDs(context.Context, []string) ([]*messageentity.Message, error) {
	return nil, nil
}

func (stub *syncMessageRepositoryStub) ListDistinctSendersBySeqRange(context.Context, string, int64, int64, string) ([]string, error) {
	return nil, nil
}

func (stub *syncMessageRepositoryStub) WithTx(any) messagerepo.MessageRepository {
	return stub
}

type syncConversationRepositoryStub struct {
	conv *conversationentity.Conversation
}

func (stub *syncConversationRepositoryStub) CreateConversation(context.Context, *conversationentity.Conversation) error {
	return nil
}

func (stub *syncConversationRepositoryStub) GetConversationSeq(context.Context, string) (int64, error) {
	return 0, nil
}

func (stub *syncConversationRepositoryStub) GetByID(context.Context, string) (*conversationentity.Conversation, error) {
	return stub.conv, nil
}

func (stub *syncConversationRepositoryStub) UpdateLatestSequence(context.Context, *conversationentity.Conversation, bool) error {
	return nil
}

func (stub *syncConversationRepositoryStub) ListByIDs(context.Context, []string) ([]*conversationentity.Conversation, error) {
	return nil, nil
}

func (stub *syncConversationRepositoryStub) WithTx(any) conversationrepo.ConversationRepository {
	return stub
}

type syncRoomUserRepositoryStub struct {
	relation *roomentity.RoomUser
	err      error
	calls    int
}

func (stub *syncRoomUserRepositoryStub) JoinRoom(user *roomentity.RoomUser) (*roomentity.RoomUser, error) {
	return user, nil
}

func (stub *syncRoomUserRepositoryStub) WithTx(any) roomrepo.RoomUserRepository { return stub }

func (stub *syncRoomUserRepositoryStub) RejoinRoom(*roomentity.RoomUser) error { return nil }

func (stub *syncRoomUserRepositoryStub) GetRelationByIDs(userId, roomId string) (*roomentity.RoomUser, error) {
	stub.calls++
	if stub.err != nil {
		return nil, stub.err
	}
	if stub.relation != nil {
		return stub.relation, nil
	}
	return &roomentity.RoomUser{UserId: userId, RoomId: roomId, Status: roomvo.Activate}, nil
}

func (stub *syncRoomUserRepositoryStub) ListActiveUserIDs(string) ([]string, error) { return nil, nil }

func (stub *syncRoomUserRepositoryStub) ListActiveRoomIDs(string) ([]string, error) { return nil, nil }

func (stub *syncRoomUserRepositoryStub) LeaveRoom(*roomentity.RoomUser, []int) error { return nil }

type syncUserRepositoryStub struct {
	users []userentity.User
}

func (stub *syncUserRepositoryStub) FindUserByPhone(string) (*userentity.User, error) {
	return nil, nil
}

func (stub *syncUserRepositoryStub) Create(*userentity.User) error { return nil }

func (stub *syncUserRepositoryStub) FindByUserID(string) (*userentity.User, error) { return nil, nil }

func (stub *syncUserRepositoryStub) FindByUsernameOrPhone(string) (*userentity.User, error) {
	return nil, nil
}

func (stub *syncUserRepositoryStub) FindByUserIDs([]string) ([]userentity.User, error) {
	return stub.users, nil
}

func (stub *syncUserRepositoryStub) UpdateOnlineTime(string, string, time.Time) error { return nil }

func (stub *syncUserRepositoryStub) UpdateOfflineTime(string, string, time.Time) error { return nil }

func (stub *syncUserRepositoryStub) WithTx(any) userrepo.UserRepository { return stub }

type syncUserCacheStub struct{}

func (stub *syncUserCacheStub) GetUserProfile(context.Context, string) (*usercache.UserProfile, bool, error) {
	return nil, false, nil
}

func (stub *syncUserCacheStub) GetUserProfiles(context.Context, []string) map[string]*usercache.UserProfile {
	return map[string]*usercache.UserProfile{}
}

func (stub *syncUserCacheStub) SetUserProfile(context.Context, *usercache.UserProfile, time.Duration) error {
	return nil
}

func (stub *syncUserCacheStub) SetUserProfileNotFound(context.Context, string, time.Duration) error {
	return nil
}

func (stub *syncUserCacheStub) DeleteUserProfiles(context.Context, []string) error { return nil }

func TestSyncMessagesReturnsMessagesAfterSeqWithPagination(t *testing.T) {
	msgRepo := &syncMessageRepositoryStub{listAfterSeqResult: []*messageentity.Message{
		messageentity.NewMessage("m4", "room1", "u2", 4, messagevo.Text, "four", "", nil),
		messageentity.NewMessage("m3", "room1", "u1", 3, messagevo.Text, "three", "", nil),
		messageentity.NewMessage("m5", "room1", "u3", 5, messagevo.Text, "five", "", nil),
	}}
	app := newSyncTestApplication(msgRepo, nil)

	msgs, nextSeq, hasMore, err := app.SyncMessages(context.Background(), "room1", "viewer", 2, 2)
	if err != nil {
		t.Fatalf("SyncMessages failed: %v", err)
	}
	if msgRepo.listAfterSeqConversationID != "room1" || msgRepo.listAfterSeqAfterSeq != 2 || msgRepo.listAfterSeqLimit != 3 {
		t.Fatalf("ListAfterSeq args mismatch: conversation=%s after=%d limit=%d", msgRepo.listAfterSeqConversationID, msgRepo.listAfterSeqAfterSeq, msgRepo.listAfterSeqLimit)
	}
	if len(msgs) != 2 {
		t.Fatalf("want 2 messages, got %d", len(msgs))
	}
	if msgs[0].Seq != 3 || msgs[1].Seq != 4 {
		t.Fatalf("wrong seqs: %+v", msgs)
	}
	if msgs[0].SenderUsername != "alice" || msgs[1].SenderUsername != "bob" {
		t.Fatalf("sender usernames not filled: %+v", msgs)
	}
	if nextSeq != 4 {
		t.Fatalf("want nextSeq=4, got %d", nextSeq)
	}
	if !hasMore {
		t.Fatal("want hasMore=true")
	}
}

func newSyncTestApplication(msgRepo *syncMessageRepositoryStub, memberCache roomcache.RoomMemberCache) *MessageApplication {
	return NewMessageApplication(
		nil, nil, nil, memberCache, &syncUserCacheStub{}, nil, nil,
		&syncConversationRepositoryStub{conv: &conversationentity.Conversation{ConversationId: "room1", Convtype: conversationvo.RoomChat, RoomId: "room1"}},
		nil, nil, nil, nil, nil, nil, nil,
		&syncUserRepositoryStub{users: []userentity.User{{UserId: "u1", UserName: "alice"}, {UserId: "u2", UserName: "bob"}}},
		msgRepo, &syncRoomUserRepositoryStub{}, nil, nil, configsForTest(),
	)
}

func TestSyncMessagesKeepsCursorWhenNoMessages(t *testing.T) {
	msgRepo := &syncMessageRepositoryStub{}
	app := newSyncTestApplication(msgRepo, nil)
	messages, nextSeq, hasMore, err := app.SyncMessages(context.Background(), "room1", "viewer", 42, 10)
	if err != nil {
		t.Fatalf("empty sync failed: %v", err)
	}
	if len(messages) != 0 || nextSeq != 42 || hasMore {
		t.Fatalf("empty sync must preserve cursor: messages=%d nextSeq=%d hasMore=%v", len(messages), nextSeq, hasMore)
	}
}

func TestNormalizeSyncLimitClampsToMaximum(t *testing.T) {
	app := &MessageApplication{config: configs.Config{Message: configs.MessageConfig{SyncDefaultLimit: 200, SyncMaxLimit: 100}}}
	if got := app.normalizeSyncLimit(0); got != 100 {
		t.Fatalf("default sync limit must not exceed max: got=%d", got)
	}
	if got := app.normalizeSyncLimit(1000); got != 100 {
		t.Fatalf("requested sync limit must be clamped: got=%d", got)
	}
}

type accessRoomMemberCacheStub struct {
	state       *roomcache.MemberState
	hit         bool
	err         error
	setState    *roomcache.MemberState
	setNotFound bool
}

func (stub *accessRoomMemberCacheStub) GetMember(context.Context, string, string) (*roomcache.MemberState, bool, error) {
	return stub.state, stub.hit, stub.err
}

func (stub *accessRoomMemberCacheStub) SetMember(_ context.Context, _, _ string, state *roomcache.MemberState) error {
	stub.setState = state
	return nil
}

func (stub *accessRoomMemberCacheStub) SetMemberIfVersionGreater(_ context.Context, _, _ string, state *roomcache.MemberState) (bool, error) {
	stub.setState = state
	return true, nil
}

func (stub *accessRoomMemberCacheStub) SetMemberNotFound(context.Context, string, string) error {
	stub.setNotFound = true
	return nil
}

func (stub *accessRoomMemberCacheStub) DeleteMember(context.Context, string, string) error {
	return nil
}

func (stub *accessRoomMemberCacheStub) GetMemberIDs(context.Context, string) ([]string, bool, error) {
	return nil, false, nil
}

func (stub *accessRoomMemberCacheStub) SetMemberIDs(context.Context, string, []string) error {
	return nil
}

func (stub *accessRoomMemberCacheStub) DeleteMemberIDs(context.Context, string) error {
	return nil
}

func TestConversationAccessUsesMemberCacheHit(t *testing.T) {
	repository := &syncRoomUserRepositoryStub{}
	cache := &accessRoomMemberCacheStub{
		state: &roomcache.MemberState{Status: roomvo.Activate},
		hit:   true,
	}
	app := newSyncTestApplication(&syncMessageRepositoryStub{}, cache)
	app.roomUserRepository = repository

	if _, _, _, err := app.SyncMessages(context.Background(), "room1", "viewer", 0, 10); err != nil {
		t.Fatalf("cache hit should allow sync: %v", err)
	}
	if repository.calls != 0 {
		t.Fatalf("cache hit should not query repository, calls=%d", repository.calls)
	}
}

func TestConversationAccessFallsBackAndWarmsCache(t *testing.T) {
	repository := &syncRoomUserRepositoryStub{}
	cache := &accessRoomMemberCacheStub{}
	app := newSyncTestApplication(&syncMessageRepositoryStub{}, cache)
	app.roomUserRepository = repository

	if _, _, _, err := app.SyncMessages(context.Background(), "room1", "viewer", 0, 10); err != nil {
		t.Fatalf("cache miss should fall back to repository: %v", err)
	}
	if repository.calls != 1 {
		t.Fatalf("cache miss should query repository once, calls=%d", repository.calls)
	}
	if cache.setState == nil || cache.setState.Status != roomvo.Activate {
		t.Fatalf("active member should be written back to cache: %+v", cache.setState)
	}
}

func TestConversationAccessRejectsNegativeCache(t *testing.T) {
	repository := &syncRoomUserRepositoryStub{}
	cache := &accessRoomMemberCacheStub{hit: true}
	app := newSyncTestApplication(&syncMessageRepositoryStub{}, cache)
	app.roomUserRepository = repository

	_, _, _, err := app.SyncMessages(context.Background(), "room1", "viewer", 0, 10)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("negative cache should reject access, err=%v", err)
	}
	if repository.calls != 0 {
		t.Fatalf("negative cache should not query repository, calls=%d", repository.calls)
	}
}

func TestConversationAccessFallsBackWhenCacheFails(t *testing.T) {
	repository := &syncRoomUserRepositoryStub{}
	cache := &accessRoomMemberCacheStub{err: errors.New("cache unavailable")}
	app := newSyncTestApplication(&syncMessageRepositoryStub{}, cache)
	app.roomUserRepository = repository

	if _, _, _, err := app.SyncMessages(context.Background(), "room1", "viewer", 0, 10); err != nil {
		t.Fatalf("cache failure should fall back to repository: %v", err)
	}
	if repository.calls != 1 {
		t.Fatalf("cache failure should query repository once, calls=%d", repository.calls)
	}
}

func TestConversationAccessRejectsLeftMember(t *testing.T) {
	repository := &syncRoomUserRepositoryStub{
		relation: &roomentity.RoomUser{Status: roomvo.Left},
	}
	cache := &accessRoomMemberCacheStub{}
	app := newSyncTestApplication(&syncMessageRepositoryStub{}, cache)
	app.roomUserRepository = repository

	_, _, _, err := app.SyncMessages(context.Background(), "room1", "viewer", 0, 10)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("left member should be rejected, err=%v", err)
	}
	if cache.setState == nil || cache.setState.Status != roomvo.Left {
		t.Fatalf("left member should write versioned state: %+v", cache.setState)
	}
}
func configsForTest() configs.Config {
	cfg := configs.Config{}
	cfg.Cache.UserProfile.TTL = 600
	cfg.Cache.UserProfile.NegativeTTL = 120
	return cfg
}
