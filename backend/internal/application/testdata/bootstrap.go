package testdata

import (
	"IM_backend/configs"
	messageapp "IM_backend/internal/application/message"
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	roomapp "IM_backend/internal/application/room"
	friendentity "IM_backend/internal/domain/friend/entity"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	uservo "IM_backend/internal/domain/user/value_object"
	"IM_backend/internal/infrastructure/id/snow"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type BootstrapResult struct {
	Users         int `json:"users"`
	Rooms         int `json:"rooms"`
	Friends       int `json:"friends"`
	Conversations int `json:"conversations"`
	Messages      int `json:"messages"`
}

type BootstrapApplication struct {
	config                 configs.Config
	db                     *gorm.DB
	redisClient            *redis.Client
	userRepository         userrepo.UserRepository
	friendRepository       friendrepo.FriendRepository
	messageRepository      messagerepo.MessageRepository
	conversationRepository messagerepo.ConversationRepository
	userConversationRepo   messagerepo.UserConversationRepository
	roomRepository         roomrepo.RoomRepository
	roomUserRepository     roomrepo.RoomUserRepository
	roomApp                *roomapp.RoomApplication
	messageApp             *messageapp.MessageApplication
}

func NewBootstrapApplication(
	config configs.Config,
	db *gorm.DB,
	redisClient *redis.Client,
	userRepository userrepo.UserRepository,
	friendRepository friendrepo.FriendRepository,
	messageRepository messagerepo.MessageRepository,
	conversationRepository messagerepo.ConversationRepository,
	userConversationRepo messagerepo.UserConversationRepository,
	roomRepository roomrepo.RoomRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	roomApp *roomapp.RoomApplication,
	messageApp *messageapp.MessageApplication,
) *BootstrapApplication {
	return &BootstrapApplication{
		config:                 config,
		db:                     db,
		redisClient:            redisClient,
		userRepository:         userRepository,
		friendRepository:       friendRepository,
		messageRepository:      messageRepository,
		conversationRepository: conversationRepository,
		userConversationRepo:   userConversationRepo,
		roomRepository:         roomRepository,
		roomUserRepository:     roomUserRepository,
		roomApp:                roomApp,
		messageApp:             messageApp,
	}
}

type bootstrapUser struct {
	ID       string
	UserName string
	NickName string
	Phone    string
	Password string
}

type bootstrapRoom struct {
	OwnerID     string
	RoomName    string
	Description string
	Members     []string
}

type avatarSource struct {
	Results []struct {
		URL string `json:"url"`
	} `json:"results"`
}

func (a *BootstrapApplication) Bootstrap(ctx context.Context) (*BootstrapResult, error) {
	if a.config.App.Env != "development" {
		return nil, errors.New("bootstrap is only available in development")
	}

	if a.redisClient != nil {
		_ = a.redisClient.FlushDB(ctx).Err()
	}

	if err := a.clearTables(ctx); err != nil {
		return nil, err
	}

	avatars := a.loadAvatars(ctx, 64)

	users := a.buildUsers()
	rooms := a.buildRooms()
	friendPairs := a.buildFriendPairs()

	userByID, err := a.createUsers(ctx, users, avatars)
	if err != nil {
		return nil, err
	}

	if err := a.createFriends(ctx, friendPairs, userByID); err != nil {
		return nil, err
	}

	roomIDs, err := a.createRooms(ctx, rooms, avatars, userByID)
	if err != nil {
		return nil, err
	}

	if err := a.createPrivateConversations(ctx, friendPairs, userByID); err != nil {
		return nil, err
	}

	if err := a.createMessages(ctx, rooms, roomIDs, friendPairs, userByID); err != nil {
		return nil, err
	}

	return &BootstrapResult{
		Users:         len(users),
		Rooms:         len(rooms),
		Friends:       len(friendPairs) * 2,
		Conversations: len(rooms) + len(friendPairs),
		Messages:      len(rooms)*12 + len(friendPairs)*8,
	}, nil
}

func (a *BootstrapApplication) clearTables(ctx context.Context) error {
	return a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tables := []string{
			"messages",
			"user_conversations",
			"conversations",
			"room_user",
			"friend_request",
			"friend",
			"room",
			"files",
			"user",
		}
		if err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			return err
		}
		for _, table := range tables {
			if err := tx.Exec("DELETE FROM " + table).Error; err != nil {
				return err
			}
		}
		return tx.Exec("SET FOREIGN_KEY_CHECKS = 1").Error
	})
}

func (a *BootstrapApplication) buildUsers() []bootstrapUser {
	users := []bootstrapUser{
		{ID: "admin", UserName: "admin", NickName: "系统管理员", Phone: "13100000000", Password: "Admin@123456"},
		{ID: "u01", UserName: "yui", NickName: "小悠", Phone: "13100000001", Password: "Demo@123456"},
		{ID: "u02", UserName: "ren", NickName: "阿仁", Phone: "13100000002", Password: "Demo@123456"},
		{ID: "u03", UserName: "mika", NickName: "美香", Phone: "13100000003", Password: "Demo@123456"},
		{ID: "u04", UserName: "sora", NickName: "空", Phone: "13100000004", Password: "Demo@123456"},
		{ID: "u05", UserName: "luna", NickName: "露娜", Phone: "13100000005", Password: "Demo@123456"},
		{ID: "u06", UserName: "haru", NickName: "春", Phone: "13100000006", Password: "Demo@123456"},
		{ID: "u07", UserName: "akira", NickName: "明", Phone: "13100000007", Password: "Demo@123456"},
		{ID: "u08", UserName: "nori", NickName: "诺里", Phone: "13100000008", Password: "Demo@123456"},
		{ID: "u09", UserName: "yuna", NickName: "由奈", Phone: "13100000009", Password: "Demo@123456"},
		{ID: "u10", UserName: "riku", NickName: "陆", Phone: "13100000010", Password: "Demo@123456"},
		{ID: "u11", UserName: "mei", NickName: "梅", Phone: "13100000011", Password: "Demo@123456"},
		{ID: "u12", UserName: "kai", NickName: "凯", Phone: "13100000012", Password: "Demo@123456"},
		{ID: "u13", UserName: "nana", NickName: "奈奈", Phone: "13100000013", Password: "Demo@123456"},
		{ID: "u14", UserName: "ryu", NickName: "龙", Phone: "13100000014", Password: "Demo@123456"},
	}

	return users
}

func (a *BootstrapApplication) buildRooms() []bootstrapRoom {
	return []bootstrapRoom{
		{
			OwnerID:     "admin",
			RoomName:    "管理员总控室",
			Description: "管理员和核心测试成员使用的房间",
			Members:     []string{"admin", "u01", "u02", "u03", "u04"},
		},
		{
			OwnerID:     "u01",
			RoomName:    "动漫闲聊屋",
			Description: "测试聊天和房间列表用",
			Members:     []string{"u01", "u05", "u06", "u07", "admin"},
		},
		{
			OwnerID:     "u02",
			RoomName:    "深夜放映室",
			Description: "测试视频和弹幕入口用",
			Members:     []string{"u02", "u08", "u09", "u10"},
		},
		{
			OwnerID:     "u03",
			RoomName:    "开发组自习室",
			Description: "测试多个成员同房间消息",
			Members:     []string{"u03", "u11", "u12", "u13", "u14"},
		},
		{
			OwnerID:     "u04",
			RoomName:    "周末茶话会",
			Description: "测试房间和成员展示",
			Members:     []string{"u04", "u01", "u10", "u12", "u14"},
		},
		{
			OwnerID:     "u05",
			RoomName:    "一起看实验室",
			Description: "测试一起看相关消息",
			Members:     []string{"u05", "u06", "u08", "u09", "u11"},
		},
	}
}

func (a *BootstrapApplication) buildFriendPairs() [][2]string {
	return [][2]string{
		{"admin", "u01"},
		{"u02", "u03"},
		{"u04", "u05"},
		{"u06", "u07"},
		{"u08", "u09"},
		{"u10", "u11"},
		{"u12", "u13"},
		{"u14", "u01"},
	}
}

func (a *BootstrapApplication) createUsers(ctx context.Context, users []bootstrapUser, avatars []string) (map[string]model.User, error) {
	userRepo := a.userRepository
	created := make(map[string]model.User, len(users))
	if len(avatars) == 0 {
		avatars = []string{"https://nekos.best/api/v2/neko/09e06604-71b8-4a50-8a1d-49672c994204.png"}
	}

	for idx, u := range users {
		hp, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		userId, err := snow.GenerateSnowID(int(a.config.App.MachineID))
		if err != nil {
			return nil, err
		}
		row := model.User{
			UserId:     userId,
			UserName:   u.UserName,
			NickName:   u.NickName,
			Password:   string(hp),
			Phone:      u.Phone,
			Avatar:     avatars[idx%len(avatars)],
			Status:     int(uservo.StatusActivate),
			OnLineTime: time.Now(),
			LoginType:  int(uservo.PhoneType),
		}
		if err := userRepo.Create(&row); err != nil {
			return nil, err
		}
		created[u.ID] = row
	}
	return created, nil
}

func (a *BootstrapApplication) createFriends(ctx context.Context, pairs [][2]string, users map[string]model.User) error {
	friends := make([]friendentity.Friend, 0, len(pairs)*2)
	for _, pair := range pairs {
		left, ok := users[pair[0]]
		if !ok {
			return fmt.Errorf("missing user %s", pair[0])
		}
		right, ok := users[pair[1]]
		if !ok {
			return fmt.Errorf("missing user %s", pair[1])
		}

		remarkLeft := right.NickName
		if pair[0] == "admin" {
			remarkLeft = "小悠"
		}
		remarkRight := left.NickName
		if pair[1] == "admin" {
			remarkRight = "管理员"
		}

		friends = append(friends, friendentity.NewFriendRelation(left.UserId, right.UserId, [2]string{remarkLeft, remarkRight})...)
	}

	return a.friendRepository.Create(friends)
}

func (a *BootstrapApplication) createRooms(ctx context.Context, rooms []bootstrapRoom, avatars []string, users map[string]model.User) (map[string]string, error) {
	roomAvatarIdx := len(avatars) / 2
	if roomAvatarIdx < 0 {
		roomAvatarIdx = 0
	}

	roomIDs := make(map[string]string, len(rooms))
	for i, room := range rooms {
		avatar := avatars[(roomAvatarIdx+i)%len(avatars)]
		owner, ok := users[room.OwnerID]
		if !ok {
			return nil, fmt.Errorf("missing owner %s", room.OwnerID)
		}
		roomDTO, err := a.roomApp.Create(ctx, owner.UserId, room.RoomName, avatar, room.Description)
		if err != nil {
			return nil, err
		}
		roomIDs[room.RoomName] = roomDTO.RoomId
		if err := a.db.WithContext(ctx).Model(&model.Conversation{}).Where("conversation_id = ?", roomDTO.RoomId).Update("latest_message_id", roomDTO.RoomId).Error; err != nil {
			return nil, err
		}

		memberSet := make(map[string]struct{}, len(room.Members))
		memberSet[room.OwnerID] = struct{}{}
		for _, memberID := range room.Members {
			memberSet[memberID] = struct{}{}
		}

		for memberID := range memberSet {
			if memberID == room.OwnerID {
				continue
			}
			member, ok := users[memberID]
			if !ok {
				return nil, fmt.Errorf("missing member %s", memberID)
			}

			inviteCode, err := a.roomApp.Invite(ctx, owner.UserId, roomDTO.RoomId)
			if err != nil {
				return nil, err
			}
			if _, _, err := a.roomApp.Join(ctx, member.UserId, inviteCode); err != nil {
				return nil, err
			}
		}

		if err := a.db.WithContext(ctx).Model(&model.Room{}).Where("room_id = ?", roomDTO.RoomId).Update("member_count", len(memberSet)).Error; err != nil {
			return nil, err
		}
	}

	return roomIDs, nil
}

func (a *BootstrapApplication) createPrivateConversations(ctx context.Context, pairs [][2]string, users map[string]model.User) error {
	for _, pair := range pairs {
		leftUser, ok := users[pair[0]]
		if !ok {
			return fmt.Errorf("missing user %s", pair[0])
		}
		rightUser, ok := users[pair[1]]
		if !ok {
			return fmt.Errorf("missing user %s", pair[1])
		}

		conversationID := messageentity.GetConversationID(leftUser.UserId, rightUser.UserId, int(messagevo.PrivateChat))
		if err := a.conversationRepository.Upsert(ctx, messageentity.BuildConversation(conversationID, leftUser.UserId, rightUser.UserId, int(messagevo.PrivateChat), 0, latestMessagePlaceholder(conversationID))); err != nil {
			return err
		}
		if err := a.userConversationRepo.CreateUserConversation(ctx, messageentity.BuildUserConversation(leftUser.UserId, conversationID, 0, 0)); err != nil {
			return err
		}
		if err := a.userConversationRepo.CreateUserConversation(ctx, messageentity.BuildUserConversation(rightUser.UserId, conversationID, 0, 0)); err != nil {
			return err
		}
	}

	return nil
}

func (a *BootstrapApplication) createMessages(ctx context.Context, rooms []bootstrapRoom, roomIDs map[string]string, pairs [][2]string, users map[string]model.User) error {
	ctx = context.WithValue(ctx, "op", "bootstrap")

	for idx, room := range rooms {
		roomID, ok := roomIDs[room.RoomName]
		if !ok {
			return fmt.Errorf("missing room %s", room.RoomName)
		}

		senders := make([]string, 0, len(room.Members)+1)
		for _, memberKey := range room.Members {
			member, ok := users[memberKey]
			if !ok {
				return fmt.Errorf("missing sender %s", memberKey)
			}
			senders = append(senders, member.UserId)
		}
		owner, ok := users[room.OwnerID]
		if !ok {
			return fmt.Errorf("missing sender %s", room.OwnerID)
		}
		senders = append(senders, owner.UserId)
		senders = uniqueStrings(senders)
		sort.Strings(senders)

		for i := 0; i < 12; i++ {
			sender := senders[i%len(senders)]
			if _, err := a.messageApp.HandleMessage(ctx, messageapp.MessageAppeDTO{
				SendId:      sender,
				RecvId:      roomID,
				ConvType:    int(messagevo.RoomChat),
				CType:       int(messagevo.Text),
				ClientMsgId: fmt.Sprintf("room-%d-%d", idx, i),
				Content:     fmt.Sprintf("%s 的测试消息 #%d", room.RoomName, i+1),
			}); err != nil {
				return err
			}
		}
	}

	for idx, pair := range pairs {
		leftUser, ok := users[pair[0]]
		if !ok {
			return fmt.Errorf("missing user %s", pair[0])
		}
		rightUser, ok := users[pair[1]]
		if !ok {
			return fmt.Errorf("missing user %s", pair[1])
		}

		for i := 0; i < 8; i++ {
			sender := leftUser
			recv := rightUser
			if i%2 == 1 {
				sender = rightUser
				recv = leftUser
			}
			if _, err := a.messageApp.HandleMessage(ctx, messageapp.MessageAppeDTO{
				SendId:      sender.UserId,
				RecvId:      recv.UserId,
				ConvType:    int(messagevo.PrivateChat),
				CType:       int(messagevo.Text),
				ClientMsgId: fmt.Sprintf("priv-%d-%d", idx, i),
				Content:     fmt.Sprintf("私聊测试消息 #%d", i+1),
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *BootstrapApplication) loadAvatars(ctx context.Context, need int) []string {
	out := make([]string, 0, need)
	seen := make(map[string]struct{}, need)
	client := &http.Client{Timeout: 5 * time.Second}
	fallbacks := []string{
		"https://nekos.best/api/v2/neko/09e06604-71b8-4a50-8a1d-49672c994204.png",
		"https://nekos.best/api/v2/neko/6f1a4cf1-4d43-4e86-8e2f-1eb0b0c2f6c4.png",
		"https://nekos.best/api/v2/neko/4ed4e1b8-2d1a-4d2b-8e8f-8ec8e7d3c111.png",
	}

	for attempts := 0; len(out) < need && attempts < need*4; attempts++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://nekos.best/api/v2/neko", nil)
		if err != nil {
			break
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var payload avatarSource
		if err := json.Unmarshal(body, &payload); err != nil {
			continue
		}
		if len(payload.Results) == 0 || payload.Results[0].URL == "" {
			continue
		}
		url := payload.Results[0].URL
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, url)
	}

	for len(out) < need {
		out = append(out, fallbacks[len(out)%len(fallbacks)])
	}

	return out
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func latestMessagePlaceholder(key string) string {
	sum := sha1.Sum([]byte(key))
	return "seed-" + hex.EncodeToString(sum[:12])
}
