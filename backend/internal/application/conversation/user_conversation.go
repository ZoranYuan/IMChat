package conversation

import (
	conversationport "IM_backend/internal/application/ports/conversation"
	conversationrepo "IM_backend/internal/application/ports/persistence/repository/conversation"
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	conversationvo "IM_backend/internal/domain/conversation/value_object"
	roomvo "IM_backend/internal/domain/room/value_object"
	"context"
	"sort"
)

type UserConvApplication struct {
	friendRepository           friendrepo.FriendRepository
	userRepository             userrepo.UserRepository
	roomRepository             roomrepo.RoomRepository
	userConversationRepository conversationrepo.UserConversationRepository
	conversationRepository     conversationrepo.ConversationRepository
	messageRepository          messagerepo.MessageRepository
}

func NewUserConvApplication(
	userConversationRepository conversationrepo.UserConversationRepository,
	conversationRepository conversationrepo.ConversationRepository,
	messageRepository messagerepo.MessageRepository,
	friendRepository friendrepo.FriendRepository,
	userRepository userrepo.UserRepository,
	roomRepository roomrepo.RoomRepository,
) *UserConvApplication {
	return &UserConvApplication{
		friendRepository:           friendRepository,
		userRepository:             userRepository,
		roomRepository:             roomRepository,
		userConversationRepository: userConversationRepository,
		conversationRepository:     conversationRepository,
		messageRepository:          messageRepository,
	}
}

func (uc *UserConvApplication) SyncLatestSequences(ctx context.Context, items []conversationport.SyncItem) error {
	if len(items) == 0 {
		return nil
	}

	conversationIDs := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.ConversationId == "" {
			continue
		}
		if _, ok := seen[item.ConversationId]; ok {
			continue
		}
		seen[item.ConversationId] = struct{}{}
		conversationIDs = append(conversationIDs, item.ConversationId)
	}

	conversations, err := uc.conversationRepository.ListByIDs(ctx, conversationIDs)
	if err != nil {
		return err
	}

	conversationByID := make(map[string]*conversationentity.Conversation, len(conversations))
	for _, conv := range conversations {
		if conv == nil {
			continue
		}
		conversationByID[conv.ConversationId] = conv
	}

	userConversations := make([]*conversationentity.UserConversation, 0, len(items))
	for _, item := range items {
		conv := conversationByID[item.ConversationId]
		if conv == nil {
			continue
		}
		userConversations = append(userConversations, conversationentity.BuildUserConversation(
			item.UserId,
			item.ConversationId,
			0,
			item.LatestSeq,
			conv.Convtype,
		))
	}

	return uc.userConversationRepository.BatchUpdateSyncSeq(ctx, userConversations)
}

func (uc *UserConvApplication) GetUserConversationsById(ctx context.Context, userId string) ([]ConversationItemDTO, error) {
	if userId == "" {
		return nil, ErrEmptyUserId
	}

	userConversations, err := uc.userConversationRepository.ListByUser(ctx, userId)
	if err != nil {
		return nil, err
	}
	if len(userConversations) == 0 {
		return []ConversationItemDTO{}, nil
	}

	conversationIDs := make([]string, 0, len(userConversations))
	for _, ucItem := range userConversations {
		if ucItem == nil || ucItem.ConversationId == "" {
			continue
		}
		conversationIDs = append(conversationIDs, ucItem.ConversationId)
	}

	conversations, err := uc.conversationRepository.ListByIDs(ctx, conversationIDs)
	if err != nil {
		return nil, err
	}

	conversationByID := make(map[string]*conversationentity.Conversation, len(conversations))
	privatePeerIDs := make([]string, 0, len(conversations))
	privateSeen := make(map[string]struct{}, len(conversations))
	roomIDs := make([]string, 0, len(conversations))
	roomSeen := make(map[string]struct{}, len(conversations))

	for _, conv := range conversations {
		if conv == nil {
			continue
		}
		conversationByID[conv.ConversationId] = conv

		switch conv.Convtype {
		case conversationvo.PrivateChat:
			peerID := conv.UserId1
			if peerID == userId {
				peerID = conv.UserId2
			}
			if peerID != "" {
				if _, ok := privateSeen[peerID]; !ok {
					privateSeen[peerID] = struct{}{}
					privatePeerIDs = append(privatePeerIDs, peerID)
				}
			}
		case conversationvo.RoomChat:
			if conv.RoomId == "" {
				continue
			}
			if _, ok := roomSeen[conv.RoomId]; ok {
				continue
			}
			roomSeen[conv.RoomId] = struct{}{}
			roomIDs = append(roomIDs, conv.RoomId)
		}
	}

	latestMessages, err := uc.messageRepository.GetLatestMessagesByConversationIDs(ctx, conversationIDs)
	if err != nil {
		return nil, err
	}

	messageByConversationID := make(map[string]LatestMessageDTO, len(latestMessages))
	for _, msg := range latestMessages {
		if msg == nil {
			continue
		}
		messageByConversationID[msg.ConversationId] = LatestMessageDTO{
			MessageId:      msg.MessageId,
			ConversationId: msg.ConversationId,
			SenderId:       msg.SendId,
			Seq:            msg.Seq,
			CType:          msg.Type,
			Content:        msg.Content,
			SendTime:       msg.SendTime,
		}
	}

	userByID := make(map[string]PeerUserDTO, len(privatePeerIDs))
	if len(privatePeerIDs) > 0 {
		users, err := uc.userRepository.FindByUserIDs(privatePeerIDs)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			userByID[user.UserId] = PeerUserDTO{
				UserId:   user.UserId,
				UserName: user.UserName,
				NickName: user.NickName,
				Avatar:   user.Avatar,
			}
		}
	}

	roomByID := make(map[string]RoomDTO, len(roomIDs))
	for _, roomID := range roomIDs {
		room, err := uc.roomRepository.FindActiveRoom(roomID, int(roomvo.Normal))
		if err != nil || room == nil {
			continue
		}
		roomByID[roomID] = RoomDTO{
			RoomId:      room.RoomId,
			RoomName:    room.RoomName,
			Avatar:      room.Avatar,
			Description: room.Description,
			MemberCount: room.MemberCount,
		}
	}

	items := make([]ConversationItemDTO, 0, len(userConversations))
	for _, userConv := range userConversations {
		if userConv == nil {
			continue
		}
		conv := conversationByID[userConv.ConversationId]
		if conv == nil {
			continue
		}

		item := ConversationItemDTO{
			ConversationId: userConv.ConversationId,
			ConvType:       int8(conv.Convtype),
			IsMuted:        userConv.IsMuted,
			Unread:         unreadCount(userConv.LastReadSeq, conv.LatestSeq),
		}

		if latestMessage, ok := messageByConversationID[userConv.ConversationId]; ok {
			latestMessage.ConvType = item.ConvType
			copy := latestMessage
			item.LastMessage = &copy
		}

		switch conv.Convtype {
		case conversationvo.PrivateChat:
			peerID := conv.UserId1
			if peerID == userId {
				peerID = conv.UserId2
			}
			item.TargetId = peerID

			peerUser := userByID[peerID]
			if relation, err := uc.friendRepository.FindRelation(userId, peerID); err == nil && relation != nil {
				peerUser.Remark = relation.Remarks
			}
			if peerUser.UserId != "" {
				item.PeerUser = &peerUser
				item.DisplayName = peerUser.Remark
				if item.DisplayName == "" {
					item.DisplayName = peerUser.NickName
				}
				if item.DisplayName == "" {
					item.DisplayName = peerUser.UserName
				}
				item.Avatar = peerUser.Avatar
			}
			if item.DisplayName == "" {
				item.DisplayName = "好友 " + peerUser.UserName
			}

		case conversationvo.RoomChat:
			item.TargetId = conv.RoomId
			if room, ok := roomByID[conv.RoomId]; ok {
				roomCopy := room
				item.Room = &roomCopy
				item.DisplayName = room.RoomName
				item.Avatar = room.Avatar
			}
			if item.DisplayName == "" {
				item.DisplayName = "房间 " + item.TargetId
			}
		}

		items = append(items, item)
	}

	// 排序
	sort.SliceStable(items, func(i, j int) bool {
		leftTime := int64(0)
		if items[i].LastMessage != nil {
			leftTime = items[i].LastMessage.SendTime
		}
		rightTime := int64(0)
		if items[j].LastMessage != nil {
			rightTime = items[j].LastMessage.SendTime
		}
		if leftTime != rightTime {
			return leftTime > rightTime
		}
		return items[i].ConversationId > items[j].ConversationId
	})

	return items, nil
}

func unreadCount(lastReadSeq int64, latestSeq int64) int64 {
	if latestSeq <= lastReadSeq {
		return 0
	}
	return latestSeq - lastReadSeq
}
