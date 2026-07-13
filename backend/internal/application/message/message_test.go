package message

import (
	"IM_backend/configs"
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	fileentity "IM_backend/internal/domain/file/entity"
	friendentity "IM_backend/internal/domain/friend/entity"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	roomentity "IM_backend/internal/domain/room/entity"
	userentity "IM_backend/internal/domain/user/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

type fakeTxManager struct{}

func (f *fakeTxManager) WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(&gorm.DB{})
}

type fakeConversationCache struct {
	mu                   sync.Mutex
	memberCheckResult    bool
	memberCheckVersion   int64
	memberCheckErr       error
	setMembersConvID     string
	setMembersUserIDs    []string
	setMembersVersion    int64
	latestSeqByConv      map[string]int64
	membersByConv        map[string][]string
	membersVersionByConv map[string]int64
}

func (c *fakeConversationCache) IsMemberWithVersion(ctx context.Context, convId, userId string) (bool, int64, error) {
	return c.memberCheckResult, c.memberCheckVersion, c.memberCheckErr
}

func (c *fakeConversationCache) SetMembers(ctx context.Context, convId string, userIds []string, version int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setMembersConvID = convId
	c.setMembersUserIDs = append([]string(nil), userIds...)
	c.setMembersVersion = version
	if c.membersByConv == nil {
		c.membersByConv = make(map[string][]string)
	}
	if c.membersVersionByConv == nil {
		c.membersVersionByConv = make(map[string]int64)
	}
	c.membersByConv[convId] = append([]string(nil), userIds...)
	c.membersVersionByConv[convId] = version
	return nil
}

func (c *fakeConversationCache) DeleteConversation(ctx context.Context, convID string) error {
	return nil
}

func (c *fakeConversationCache) IncrConvLatestSeq(ctx context.Context, convId string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.latestSeqByConv == nil {
		c.latestSeqByConv = make(map[string]int64)
	}
	c.latestSeqByConv[convId]++
	return c.latestSeqByConv[convId], nil
}

func (c *fakeConversationCache) GetConvLatestSeq(ctx context.Context, convId string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.latestSeqByConv == nil {
		return 0, nil
	}
	return c.latestSeqByConv[convId], nil
}

func (c *fakeConversationCache) SetConvSeq(ctx context.Context, convId string, seq int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.latestSeqByConv == nil {
		c.latestSeqByConv = make(map[string]int64)
	}
	c.latestSeqByConv[convId] = seq
	return nil
}

func (c *fakeConversationCache) AddMember(ctx context.Context, convId, userId string, version int64) error {
	return nil
}
func (c *fakeConversationCache) RemoveMember(ctx context.Context, convId, userId string, version int64) error {
	return nil
}

func (c *fakeConversationCache) GetMembersWithVersion(ctx context.Context, convId string) ([]string, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.membersByConv == nil {
		return nil, 0, nil
	}
	return append([]string(nil), c.membersByConv[convId]...), c.membersVersionByConv[convId], nil
}

func (c *fakeConversationCache) SetDedupEntry(ctx context.Context, clientMsgId, messageId string, ttl time.Duration) (bool, error) {
	return true, nil
}

func (c *fakeConversationCache) GetDedupEntry(ctx context.Context, clientMsgId string) (string, error) {
	return "", nil
}

type fakeFriendRepository struct {
	relation *friendentity.Friend
	err      error
}

func (r *fakeFriendRepository) Create(domains []friendentity.Friend) error { return nil }
func (r *fakeFriendRepository) FindRelation(userId, friendId string) (*friendentity.Friend, error) {
	return r.relation, r.err
}
func (r *fakeFriendRepository) WithTx(tx *gorm.DB) friendrepo.FriendRepository { return r }
func (r *fakeFriendRepository) GetUserFriendList(userId string, status int) ([]friendentity.Friend, error) {
	return nil, nil
}

type fakeUserRepository struct {
	modelUsers  map[string]*model.User
	domainUsers map[string]userentity.User
}

func (r *fakeUserRepository) FindUserByPhone(string) (*model.User, error) { return nil, nil }
func (r *fakeUserRepository) Create(*model.User) error                    { return nil }
func (r *fakeUserRepository) FindByUserID(userId string) (*model.User, error) {
	if r.modelUsers == nil {
		return nil, errors.New("not found")
	}
	u, ok := r.modelUsers[userId]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}
func (r *fakeUserRepository) FindByUsernameOrPhone(keyword string) (*model.User, error) {
	return nil, nil
}
func (r *fakeUserRepository) FindByUserIDs(userIds []string) ([]userentity.User, error) {
	res := make([]userentity.User, 0, len(userIds))
	for _, id := range userIds {
		if u, ok := r.domainUsers[id]; ok {
			res = append(res, u)
		}
	}
	return res, nil
}
func (r *fakeUserRepository) UpdateByUserIDAndPhone(string, string, map[string]interface{}) error {
	return nil
}
func (r *fakeUserRepository) WithTx(tx *gorm.DB) userrepo.UserRepository { return r }

type fakeConversationRepository struct {
	byID map[string]*messageentity.Conversation
	list []*messageentity.Conversation
}

func (r *fakeConversationRepository) CreateConversation(ctx context.Context, conv *messageentity.Conversation) error {
	return nil
}
func (r *fakeConversationRepository) GetConversationSeq(ctx context.Context, conversationID string) (int64, error) {
	return 0, nil
}
func (r *fakeConversationRepository) GetByID(ctx context.Context, conversationId string) (*messageentity.Conversation, error) {
	if r.byID == nil {
		return nil, nil
	}
	return r.byID[conversationId], nil
}
func (r *fakeConversationRepository) Upsert(ctx context.Context, domain *messageentity.Conversation) error {
	return nil
}
func (r *fakeConversationRepository) ListByIDs(ctx context.Context, ids []string) ([]*messageentity.Conversation, error) {
	if r.list != nil {
		return r.list, nil
	}
	return []*messageentity.Conversation{}, nil
}
func (r *fakeConversationRepository) WithTx(tx any) messagerepo.ConversationRepository { return r }

type fakeUserConversationRepository struct {
	byKey            map[string]*messageentity.UserConversation
	listByUserResult []*messageentity.UserConversation
	updatedReadSeq   []*messageentity.UserConversation
	updatedSyncSeq   []*messageentity.UserConversation
}

func (r *fakeUserConversationRepository) CreateUserConversation(ctx context.Context, uc *messageentity.UserConversation) error {
	return nil
}
func (r *fakeUserConversationRepository) GetUsersByConversationID(ctx context.Context, conversationID string) ([]string, error) {
	return nil, nil
}
func (r *fakeUserConversationRepository) BatchUpdateSyncSeq(ctx context.Context, ucs []*messageentity.UserConversation) error {
	r.updatedSyncSeq = append(r.updatedSyncSeq, ucs...)
	return nil
}
func (r *fakeUserConversationRepository) GetUserConversation(ctx context.Context, userId string, conversationId string) (*messageentity.UserConversation, error) {
	if r.byKey == nil {
		return nil, errors.New("not found")
	}
	uc, ok := r.byKey[userId+":"+conversationId]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *uc
	return &cp, nil
}
func (r *fakeUserConversationRepository) UpdateSyncSeq(ctx context.Context, uc *messageentity.UserConversation) error {
	cp := *uc
	r.updatedSyncSeq = append(r.updatedSyncSeq, &cp)
	if r.byKey == nil {
		r.byKey = make(map[string]*messageentity.UserConversation)
	}
	r.byKey[uc.UserId+":"+uc.ConversationId] = &cp
	return nil
}
func (r *fakeUserConversationRepository) UpdateReadSeq(ctx context.Context, uc *messageentity.UserConversation) error {
	cp := *uc
	r.updatedReadSeq = append(r.updatedReadSeq, &cp)
	if r.byKey == nil {
		r.byKey = make(map[string]*messageentity.UserConversation)
	}
	r.byKey[uc.UserId+":"+uc.ConversationId] = &cp
	return nil
}
func (r *fakeUserConversationRepository) ListByUser(ctx context.Context, userId string) ([]*messageentity.UserConversation, error) {
	return r.listByUserResult, nil
}
func (r *fakeUserConversationRepository) WithTx(tx any) messagerepo.UserConversationRepository {
	return r
}

type fakeMessageRepository struct {
	saved                 []*messageentity.Message
	historyMessages       []*messageentity.Message
	latestMessages        []*messageentity.Message
	historyConversationID string
	historyMaxSeq         int64
	historyLimit          int
}

func (r *fakeMessageRepository) Save(ctx context.Context, msg *messageentity.Message) error {
	cp := *msg
	r.saved = append(r.saved, &cp)
	return nil
}
func (r *fakeMessageRepository) GetHistoryMessage(ctx context.Context, conversationId string, minSeq int64, limit int) ([]*messageentity.Message, error) {
	r.historyConversationID = conversationId
	r.historyMaxSeq = minSeq
	r.historyLimit = limit
	return r.historyMessages, nil
}
func (r *fakeMessageRepository) GetMessagesBySendTime(ctx context.Context, conversationId string, startTime int64, endTime int64, limit int) ([]*messageentity.Message, error) {
	return nil, nil
}
func (r *fakeMessageRepository) GetDanmakuByRoomVideo(ctx context.Context, conversationId string, videoId string, startTime int64, endTime int64, limit int) ([]*messageentity.Message, error) {
	return nil, nil
}
func (r *fakeMessageRepository) GetRoomVideoHistory(ctx context.Context, conversationId string, limit int) ([]*messageentity.Message, error) {
	return nil, nil
}
func (r *fakeMessageRepository) CountRoomVideoMessages(ctx context.Context, conversationId string, videoId string) (int64, error) {
	return 0, nil
}
func (r *fakeMessageRepository) GetLatestMessagesByConversationIDs(ctx context.Context, conversationIDs []string) ([]*messageentity.Message, error) {
	return r.latestMessages, nil
}
func (r *fakeMessageRepository) WithTx(tx any) messagerepo.MessageRepository { return r }

type fakeMessageOutboxRepository struct {
	created []*messageentity.MessageOutbox
}

func (r *fakeMessageOutboxRepository) Create(ctx context.Context, outbox *messageentity.MessageOutbox) error {
	cp := *outbox
	cp.Payload = append([]byte(nil), outbox.Payload...)
	r.created = append(r.created, &cp)
	return nil
}
func (r *fakeMessageOutboxRepository) ClaimPending(ctx context.Context, now time.Time, staleBefore time.Time, limit int) ([]*messageentity.MessageOutbox, error) {
	return nil, nil
}
func (r *fakeMessageOutboxRepository) MarkSent(ctx context.Context, id string, sentAt time.Time) error {
	return nil
}
func (r *fakeMessageOutboxRepository) MarkRetry(ctx context.Context, id string, nextRetryAt time.Time, lastError string) error {
	return nil
}
func (r *fakeMessageOutboxRepository) WithTx(tx any) messagerepo.MessageOutboxRepository { return r }

type fakeFileRepository struct{}

func (r *fakeFileRepository) Save(ctx context.Context, file *fileentity.File) error { return nil }
func (r *fakeFileRepository) GetByID(ctx context.Context, fileId string) (*fileentity.File, error) {
	return nil, nil
}

type fakeMessageImageRepository struct {
	created   []*messageentity.MessageImage
	batchData map[string]*messageentity.MessageImage
}

func (r *fakeMessageImageRepository) Create(ctx context.Context, item *messageentity.MessageImage) error {
	cp := *item
	r.created = append(r.created, &cp)
	return nil
}
func (r *fakeMessageImageRepository) GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageImage, error) {
	return nil, nil
}
func (r *fakeMessageImageRepository) BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageImage, error) {
	if r.batchData != nil {
		return r.batchData, nil
	}
	return map[string]*messageentity.MessageImage{}, nil
}
func (r *fakeMessageImageRepository) WithTx(tx any) messagerepo.MessageImageRepository { return r }

type fakeMessageFileRepository struct {
	created   []*messageentity.MessageFile
	batchData map[string]*messageentity.MessageFile
}

func (r *fakeMessageFileRepository) Create(ctx context.Context, item *messageentity.MessageFile) error {
	cp := *item
	r.created = append(r.created, &cp)
	return nil
}
func (r *fakeMessageFileRepository) GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageFile, error) {
	return nil, nil
}
func (r *fakeMessageFileRepository) BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageFile, error) {
	if r.batchData != nil {
		return r.batchData, nil
	}
	return map[string]*messageentity.MessageFile{}, nil
}
func (r *fakeMessageFileRepository) WithTx(tx any) messagerepo.MessageFileRepository { return r }

type fakeMessageStickerRepository struct {
	created   []*messageentity.MessageSticker
	batchData map[string]*messageentity.MessageSticker
}

func (r *fakeMessageStickerRepository) Create(ctx context.Context, item *messageentity.MessageSticker) error {
	cp := *item
	r.created = append(r.created, &cp)
	return nil
}
func (r *fakeMessageStickerRepository) GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageSticker, error) {
	return nil, nil
}
func (r *fakeMessageStickerRepository) BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageSticker, error) {
	if r.batchData != nil {
		return r.batchData, nil
	}
	return map[string]*messageentity.MessageSticker{}, nil
}
func (r *fakeMessageStickerRepository) WithTx(tx any) messagerepo.MessageStickerRepository { return r }

type fakeMessageVideoRepository struct {
	created   []*messageentity.MessageVideo
	batchData map[string]*messageentity.MessageVideo
}

func (r *fakeMessageVideoRepository) Create(ctx context.Context, item *messageentity.MessageVideo) error {
	cp := *item
	r.created = append(r.created, &cp)
	return nil
}
func (r *fakeMessageVideoRepository) GetByMessageID(ctx context.Context, messageId string) (*messageentity.MessageVideo, error) {
	return nil, nil
}
func (r *fakeMessageVideoRepository) BatchGetByMessageIDs(ctx context.Context, messageIds []string) (map[string]*messageentity.MessageVideo, error) {
	if r.batchData != nil {
		return r.batchData, nil
	}
	return map[string]*messageentity.MessageVideo{}, nil
}
func (r *fakeMessageVideoRepository) WithTx(tx any) messagerepo.MessageVideoRepository { return r }

type fakeRoomRepository struct{}

func (r *fakeRoomRepository) Create(domain *roomentity.Room) error           { return nil }
func (r *fakeRoomRepository) WithTx(tx *gorm.DB) roomrepo.RoomRepository     { return r }
func (r *fakeRoomRepository) UpdateRoomVersion(roomId string) (int64, error) { return 0, nil }
func (r *fakeRoomRepository) FindActiveRoom(roomId string, status int) (*roomentity.Room, error) {
	return nil, nil
}

type fakeRoomUserRepository struct{}

func (r *fakeRoomUserRepository) Create(*roomentity.RoomUser) (*roomentity.RoomUser, error) {
	return nil, nil
}
func (r *fakeRoomUserRepository) WithTx(*gorm.DB) roomrepo.RoomUserRepository { return r }
func (r *fakeRoomUserRepository) GetRelationByIDs(string, string) (*roomentity.RoomUser, error) {
	return nil, nil
}
func (r *fakeRoomUserRepository) ListActiveUserIDs(roomId string) ([]string, error) { return nil, nil }
func (r *fakeRoomUserRepository) JoinRoom(ru *roomentity.RoomUser) (*roomentity.RoomUser, error) {
	return ru, nil
}
func (r *fakeRoomUserRepository) LeaveRoom(*roomentity.RoomUser, []int) error        { return nil }

type fakeTaskManager struct {
	mu                        sync.Mutex
	sentMessages              []protocol.MessageEvent
	publishedReadAcks         []protocol.MessageReadAckEvent
	publishedConversationSync []protocol.ConversationSyncSeqEvent
}

func (m *fakeTaskManager) SendMessage(ctx context.Context, topic string, key string, event protocol.MessageEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := event
	m.sentMessages = append(m.sentMessages, cp)
	return nil
}
func (m *fakeTaskManager) PublishMessageReadAck(ctx context.Context, topic string, key string, event protocol.MessageReadAckEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := event
	m.publishedReadAcks = append(m.publishedReadAcks, cp)
	return nil
}
func (m *fakeTaskManager) SendConversationSyncSeq(ctx context.Context, topic string, key string, event protocol.ConversationSyncSeqEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := event
	m.publishedConversationSync = append(m.publishedConversationSync, cp)
	return nil
}

func newTestMessageApp() (*MessageApplication, *fakeMessageRepository, *fakeUserConversationRepository, *fakeConversationRepository, *fakeMessageOutboxRepository, *fakeTaskManager, *fakeConversationCache, *fakeUserRepository, *fakeMessageImageRepository, *fakeMessageFileRepository, *fakeMessageStickerRepository, *fakeMessageVideoRepository) {
	msgRepo := &fakeMessageRepository{}
	userConvRepo := &fakeUserConversationRepository{
		byKey: map[string]*messageentity.UserConversation{},
	}
	convRepo := &fakeConversationRepository{
		byID: map[string]*messageentity.Conversation{},
	}
	outboxRepo := &fakeMessageOutboxRepository{}
	taskManager := &fakeTaskManager{}
	cache := &fakeConversationCache{
		latestSeqByConv: make(map[string]int64),
	}
	imageRepo := &fakeMessageImageRepository{}
	fileMsgRepo := &fakeMessageFileRepository{}
	stickerRepo := &fakeMessageStickerRepository{}
	videoRepo := &fakeMessageVideoRepository{}
	userRepo := &fakeUserRepository{
		modelUsers: map[string]*model.User{
			"u1": {UserId: "u1", UserName: "alice", Avatar: "avatar-u1"},
			"u2": {UserId: "u2", UserName: "bob", Avatar: "avatar-u2"},
		},
		domainUsers: map[string]userentity.User{
			"u1": {UserId: "u1", UserName: "alice", Avatar: "avatar-u1"},
			"u2": {UserId: "u2", UserName: "bob", Avatar: "avatar-u2"},
			"u3": {UserId: "u3", UserName: "charlie", Avatar: "avatar-u3"},
		},
	}
	app := NewMessageApplication(
		testConfig(),
		cache,
		&fakeTxManager{},
		taskManager,
		userConvRepo,
		convRepo,
		outboxRepo,
		&fakeFriendRepository{relation: &friendentity.Friend{UserId: "u1", FriendUserId: "u2", Status: 1}},
		&fakeFileRepository{},
		imageRepo,
		fileMsgRepo,
		stickerRepo,
		videoRepo,
		userRepo,
		msgRepo,
		&fakeRoomUserRepository{},
		&fakeRoomRepository{},
	)
	return app, msgRepo, userConvRepo, convRepo, outboxRepo, taskManager, cache, userRepo, imageRepo, fileMsgRepo, stickerRepo, videoRepo
}

func testConfig() configs.Config {
	return configs.Config{
		App: configs.App{MachineID: 1},
	}
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func TestHandleMessageStoresMessageAndCreatesOutbox(t *testing.T) {
	app, msgRepo, userConvRepo, convRepo, outboxRepo, _, cache, _, _, _, _, _ := newTestMessageApp()
	ctx := context.WithValue(context.Background(), "op", "msg")

	dto, err := app.HandleMessage(ctx, MessageAppeDTO{
		ClientMsgId: "client-1",
		SendId:      "u1",
		RecvId:      "u2",
		ConvType:    int(messagevo.PrivateChat),
		CType:       int(messagevo.Text),
		Content:     "hello",
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if dto.Status != string(protocol.AckStatusSent) {
		t.Fatalf("unexpected status: %s", dto.Status)
	}
	if len(msgRepo.saved) != 1 {
		t.Fatalf("expected 1 saved message, got %d", len(msgRepo.saved))
	}
	if len(userConvRepo.updatedReadSeq) == 0 || len(userConvRepo.updatedSyncSeq) == 0 {
		t.Fatalf("expected sender conversation seq to be updated")
	}
	if convRepo == nil || cache == nil {
		t.Fatalf("test app not initialized correctly")
	}
	if len(outboxRepo.created) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outboxRepo.created))
	}
	var event protocol.MessageEvent
	if err := json.Unmarshal(outboxRepo.created[0].Payload, &event); err != nil {
		t.Fatalf("unmarshal outbox payload: %v", err)
	}
	if got := outboxRepo.created[0]; got.EventType != string(protocol.EventTypeMessage) || got.Topic != string(protocol.EventTypeMessage) || got.MessageKey != event.ConversationId {
		t.Fatalf("unexpected outbox metadata: %+v", got)
	}
	if event.ClientMsgId != "client-1" || event.SendId != "u1" || event.RecvId != "u2" || event.Content != "hello" {
		t.Fatalf("unexpected outbox payload: %+v", event)
	}
}

func TestHandleMessagePersistsMediaSubtables(t *testing.T) {
	cases := []struct {
		name       string
		cType      messagevo.CType
		dto        MessageAppeDTO
		assertRepo func(*fakeMessageImageRepository, *fakeMessageFileRepository, *fakeMessageStickerRepository, *fakeMessageVideoRepository, *testing.T)
	}{
		{
			name:  "image",
			cType: messagevo.Image,
			dto: MessageAppeDTO{
				ClientMsgId: "client-img",
				SendId:      "u1",
				RecvId:      "u2",
				ConvType:    int(messagevo.PrivateChat),
				CType:       int(messagevo.Image),
				Content:     "[图片]",
				FileId:      "file-img",
				MediaURL:    "https://example.com/image.jpg",
				FileName:    "image.jpg",
				FileSize:    1234,
				Width:       640,
				Height:      480,
			},
			assertRepo: func(imageRepo *fakeMessageImageRepository, fileRepo *fakeMessageFileRepository, stickerRepo *fakeMessageStickerRepository, videoRepo *fakeMessageVideoRepository, t *testing.T) {
				if len(imageRepo.created) != 1 || len(fileRepo.created) != 0 || len(stickerRepo.created) != 0 || len(videoRepo.created) != 0 {
					t.Fatalf("unexpected media repo writes: image=%d file=%d sticker=%d video=%d", len(imageRepo.created), len(fileRepo.created), len(stickerRepo.created), len(videoRepo.created))
				}
				if got := imageRepo.created[0]; got.FileId != "file-img" || got.URL != "https://example.com/image.jpg" || got.Width != 640 || got.Height != 480 {
					t.Fatalf("unexpected image media row: %+v", got)
				}
			},
		},
		{
			name:  "file",
			cType: messagevo.File,
			dto: MessageAppeDTO{
				ClientMsgId: "client-file",
				SendId:      "u1",
				RecvId:      "u2",
				ConvType:    int(messagevo.PrivateChat),
				CType:       int(messagevo.File),
				Content:     "[文件] handbook.pdf",
				FileId:      "file-doc",
				MediaURL:    "https://example.com/handbook.pdf",
				FileName:    "handbook.pdf",
				FileSize:    2048,
			},
			assertRepo: func(imageRepo *fakeMessageImageRepository, fileRepo *fakeMessageFileRepository, stickerRepo *fakeMessageStickerRepository, videoRepo *fakeMessageVideoRepository, t *testing.T) {
				if len(imageRepo.created) != 0 || len(fileRepo.created) != 1 || len(stickerRepo.created) != 0 || len(videoRepo.created) != 0 {
					t.Fatalf("unexpected media repo writes: image=%d file=%d sticker=%d video=%d", len(imageRepo.created), len(fileRepo.created), len(stickerRepo.created), len(videoRepo.created))
				}
				if got := fileRepo.created[0]; got.FileId != "file-doc" || got.FileName != "handbook.pdf" || got.URL != "https://example.com/handbook.pdf" {
					t.Fatalf("unexpected file media row: %+v", got)
				}
			},
		},
		{
			name:  "video",
			cType: messagevo.Video,
			dto: MessageAppeDTO{
				ClientMsgId: "client-video",
				SendId:      "u1",
				RecvId:      "u2",
				ConvType:    int(messagevo.PrivateChat),
				CType:       int(messagevo.Video),
				Content:     "[视频]",
				FileId:      "file-video",
				MediaURL:    "https://example.com/video.mp4",
				FileName:    "video.mp4",
				FileSize:    4096,
				Width:       1280,
				Height:      720,
				DurationMs:  func() *int64 { v := int64(56000); return &v }(),
			},
			assertRepo: func(imageRepo *fakeMessageImageRepository, fileRepo *fakeMessageFileRepository, stickerRepo *fakeMessageStickerRepository, videoRepo *fakeMessageVideoRepository, t *testing.T) {
				if len(imageRepo.created) != 0 || len(fileRepo.created) != 0 || len(stickerRepo.created) != 0 || len(videoRepo.created) != 1 {
					t.Fatalf("unexpected media repo writes: image=%d file=%d sticker=%d video=%d", len(imageRepo.created), len(fileRepo.created), len(stickerRepo.created), len(videoRepo.created))
				}
				if got := videoRepo.created[0]; got.FileId != "file-video" || got.URL != "https://example.com/video.mp4" || got.DurationMs != 56000 {
					t.Fatalf("unexpected video media row: %+v", got)
				}
			},
		},
		{
			name:  "sticker",
			cType: messagevo.Sticker,
			dto: MessageAppeDTO{
				ClientMsgId: "client-sticker",
				SendId:      "u1",
				RecvId:      "u2",
				ConvType:    int(messagevo.PrivateChat),
				CType:       int(messagevo.Sticker),
				Content:     "[表情包]",
				StickerId:   "sticker-1",
				PackId:      "pack-1",
				MediaURL:    "https://example.com/sticker.webp",
				Width:       200,
				Height:      200,
			},
			assertRepo: func(imageRepo *fakeMessageImageRepository, fileRepo *fakeMessageFileRepository, stickerRepo *fakeMessageStickerRepository, videoRepo *fakeMessageVideoRepository, t *testing.T) {
				if len(imageRepo.created) != 0 || len(fileRepo.created) != 0 || len(stickerRepo.created) != 1 || len(videoRepo.created) != 0 {
					t.Fatalf("unexpected media repo writes: image=%d file=%d sticker=%d video=%d", len(imageRepo.created), len(fileRepo.created), len(stickerRepo.created), len(videoRepo.created))
				}
				if got := stickerRepo.created[0]; got.StickerId != "sticker-1" || got.PackId != "pack-1" || got.URL != "https://example.com/sticker.webp" {
					t.Fatalf("unexpected sticker media row: %+v", got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, _, _, _, _, _, _, _, imageRepo, fileRepo, stickerRepo, videoRepo := newTestMessageApp()
			ctx := context.WithValue(context.Background(), "op", "msg")

			dto := tc.dto
			_, err := app.HandleMessage(ctx, dto)
			if err != nil {
				t.Fatalf("HandleMessage() error = %v", err)
			}
			tc.assertRepo(imageRepo, fileRepo, stickerRepo, videoRepo, t)
		})
	}
}

func TestHandleMessageReadAckWritesOutboxAndReadSeq(t *testing.T) {
	app, _, userConvRepo, convRepo, outboxRepo, _, _, _, _, _, _, _ := newTestMessageApp()
	convRepo.byID["conv-room"] = &messageentity.Conversation{
		ConversationId: "conv-room",
		Convtype:       messagevo.RoomChat,
		RoomId:         "room-1",
	}
	userConvRepo.byKey["u2:conv-room"] = &messageentity.UserConversation{
		UserId:         "u2",
		ConversationId: "conv-room",
		LastReadSeq:    8,
		LatestSyncSeq:  8,
	}

	err := app.HandleMessageReadAck(context.Background(), "u2", "conv-room", 12, "u1")
	if err != nil {
		t.Fatalf("HandleMessageReadAck() error = %v", err)
	}
	if len(userConvRepo.updatedReadSeq) != 1 {
		t.Fatalf("expected read seq update, got %d", len(userConvRepo.updatedReadSeq))
	}
	if got := userConvRepo.updatedReadSeq[0].LastReadSeq; got != 12 {
		t.Fatalf("unexpected last read seq: %d", got)
	}
	if len(outboxRepo.created) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outboxRepo.created))
	}
	var event protocol.MessageReadAckEvent
	if err := json.Unmarshal(outboxRepo.created[0].Payload, &event); err != nil {
		t.Fatalf("unmarshal outbox payload: %v", err)
	}
	if event.UserId != "u2" || event.ConversationId != "conv-room" || event.LastReadSeq != 12 || event.SenderId != "u1" || event.Avatar != "avatar-u2" {
		t.Fatalf("unexpected read ack event: %+v", event)
	}
}

func TestHandleMessageReadAckUpdatesReadSeqWithoutNotifyForSelfSender(t *testing.T) {
	app, _, userConvRepo, convRepo, outboxRepo, _, _, _, _, _, _, _ := newTestMessageApp()
	convRepo.byID["conv-room"] = &messageentity.Conversation{
		ConversationId: "conv-room",
		Convtype:       messagevo.PrivateChat,
		UserId1:        "u1",
		UserId2:        "u2",
	}
	userConvRepo.byKey["u1:conv-room"] = &messageentity.UserConversation{
		UserId:         "u1",
		ConversationId: "conv-room",
		LastReadSeq:    8,
		LatestSyncSeq:  8,
	}

	err := app.HandleMessageReadAck(context.Background(), "u1", "conv-room", 12, "u1")
	if err != nil {
		t.Fatalf("HandleMessageReadAck() error = %v", err)
	}
	if len(userConvRepo.updatedReadSeq) != 1 {
		t.Fatalf("expected read seq update, got %d", len(userConvRepo.updatedReadSeq))
	}
	if got := userConvRepo.updatedReadSeq[0].LastReadSeq; got != 12 {
		t.Fatalf("unexpected last read seq: %d", got)
	}
	if len(outboxRepo.created) != 0 {
		t.Fatalf("expected no outbox when sender reads own message, got %d", len(outboxRepo.created))
	}
}

func TestGetHistoryMessagesUsesCursorAndSortsAscending(t *testing.T) {
	app, msgRepo, userConvRepo, _, _, _, _, _, _, _, _, _ := newTestMessageApp()
	userConvRepo.byKey["u1:conv-1"] = &messageentity.UserConversation{
		UserId:         "u1",
		ConversationId: "conv-1",
		LastReadSeq:    12,
		LatestSyncSeq:  12,
	}
	msgRepo.historyMessages = []*messageentity.Message{
		{MessageId: "m12", ConversationId: "conv-1", SendId: "u2", Seq: 12, Type: messagevo.Text, Content: "b", SendTime: 200},
		{MessageId: "m11", ConversationId: "conv-1", SendId: "u1", Seq: 11, Type: messagevo.Text, Content: "a", SendTime: 100},
	}

	msgs, nextCursor, hasMore, err := app.GetHistoryMessages(context.Background(), "conv-1", "u1", 2, 0)
	if err != nil {
		t.Fatalf("GetHistoryMessages() error = %v", err)
	}
	if msgRepo.historyMaxSeq != 13 || msgRepo.historyLimit != 2 {
		t.Fatalf("unexpected history query args: maxSeq=%d limit=%d", msgRepo.historyMaxSeq, msgRepo.historyLimit)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Seq != 11 || msgs[1].Seq != 12 {
		t.Fatalf("messages should be ascending by seq: %+v", msgs)
	}
	if nextCursor != 11 || !hasMore {
		t.Fatalf("unexpected pagination result nextCursor=%d hasMore=%v", nextCursor, hasMore)
	}
}

func TestGetOfflineMessagesCalculatesUnreadAndUpdatesSyncSeq(t *testing.T) {
	app, msgRepo, userConvRepo, convRepo, outboxRepo, _, _, _, _, _, _, _ := newTestMessageApp()
	userConvRepo.listByUserResult = []*messageentity.UserConversation{
		{UserId: "u1", ConversationId: "conv-1", LastReadSeq: 8, LatestSyncSeq: 10},
		{UserId: "u1", ConversationId: "conv-2", LastReadSeq: 3, LatestSyncSeq: 5},
	}
	convRepo.list = []*messageentity.Conversation{
		{ConversationId: "conv-1", Convtype: messagevo.PrivateChat, UserId1: "u1", UserId2: "u2", LatestSeq: 10},
		{ConversationId: "conv-2", Convtype: messagevo.PrivateChat, UserId1: "u1", UserId2: "u3", LatestSeq: 9},
	}
	msgRepo.latestMessages = []*messageentity.Message{
		{MessageId: "m1", ConversationId: "conv-1", SendId: "u2", Seq: 10, Type: messagevo.Text, Content: "latest-1", SendTime: 1000},
		{MessageId: "m2", ConversationId: "conv-2", SendId: "u3", Seq: 9, Type: messagevo.Text, Content: "latest-2", SendTime: 2000},
	}

	msgs, unreadMap, err := app.GetOfflineMessages(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetOfflineMessages() error = %v", err)
	}
	if unreadMap["conv-1"] != 2 || unreadMap["conv-2"] != 6 {
		t.Fatalf("unexpected unread map: %#v", unreadMap)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 offline messages, got %d", len(msgs))
	}
	if msgs[0].DisplayName == "" || msgs[0].Avatar == "" {
		t.Fatalf("expected offline messages to be decorated with conversation metadata: %+v", msgs[0])
	}
	if len(outboxRepo.created) != 0 {
		t.Fatalf("expected no sync outbox events, got %d", len(outboxRepo.created))
	}
	if len(userConvRepo.updatedSyncSeq) != 1 {
		t.Fatalf("expected one sync seq update, got %d", len(userConvRepo.updatedSyncSeq))
	}
	if got := userConvRepo.updatedSyncSeq[0]; got.UserId != "u1" || got.ConversationId != "conv-2" || got.LatestSyncSeq != 9 {
		t.Fatalf("unexpected sync seq update: %+v", got)
	}
}

func TestGetOfflineMessagesOnReconnectOnlyUpdatesLaggingSyncSeqs(t *testing.T) {
	app, _, userConvRepo, convRepo, outboxRepo, _, _, _, _, _, _, _ := newTestMessageApp()
	userConvRepo.listByUserResult = []*messageentity.UserConversation{
		{UserId: "u1", ConversationId: "conv-1", LastReadSeq: 11, LatestSyncSeq: 12},
		{UserId: "u1", ConversationId: "conv-2", LastReadSeq: 4, LatestSyncSeq: 6},
	}
	convRepo.list = []*messageentity.Conversation{
		{ConversationId: "conv-1", Convtype: messagevo.PrivateChat, UserId1: "u1", UserId2: "u2", LatestSeq: 12},
		{ConversationId: "conv-2", Convtype: messagevo.PrivateChat, UserId1: "u1", UserId2: "u3", LatestSeq: 9},
	}

	msgs, unreadMap, err := app.GetOfflineMessages(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetOfflineMessages() error = %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no latest messages when repo returns none, got %d", len(msgs))
	}
	if unreadMap["conv-1"] != 1 || unreadMap["conv-2"] != 5 {
		t.Fatalf("unexpected unread map: %#v", unreadMap)
	}
	if len(outboxRepo.created) != 0 {
		t.Fatalf("expected no sync outbox events, got %d", len(outboxRepo.created))
	}
	if len(userConvRepo.updatedSyncSeq) != 1 {
		t.Fatalf("expected one lagging sync seq update, got %d", len(userConvRepo.updatedSyncSeq))
	}
	if got := userConvRepo.updatedSyncSeq[0]; got.UserId != "u1" || got.ConversationId != "conv-2" || got.LatestSyncSeq != 9 {
		t.Fatalf("unexpected reconnect sync update: %+v", got)
	}
}
