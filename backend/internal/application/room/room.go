package room

import (
	idport "IM_backend/internal/application/ports/id"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	conversationrepo "IM_backend/internal/application/ports/persistence/repository/conversation"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	conversationvo "IM_backend/internal/domain/conversation/value_object"
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"context"
	"errors"
	"log"
	"time"
)

type RoomApplication struct {
	roomRepository             roomrepo.RoomRepository
	roomUserRepository         roomrepo.RoomUserRepository
	userConversationRepository conversationrepo.UserConversationRepository
	conversationRepository     conversationrepo.ConversationRepository
	roomCache                  roomcache.RoomCache
	roomMemberCache            roomcache.RoomMemberCache
	txManager                  txmanager.TxManager
	idGenerator                idport.Generator
}

func NewRoomApplication(roomRepository roomrepo.RoomRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	userConversationRepository conversationrepo.UserConversationRepository,
	conversationRepository conversationrepo.ConversationRepository,
	roomCache roomcache.RoomCache,
	roomMemberCache roomcache.RoomMemberCache,
	txManager txmanager.TxManager,
	idGenerator idport.Generator,
) *RoomApplication {
	return &RoomApplication{
		roomRepository:             roomRepository,
		roomUserRepository:         roomUserRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
		roomCache:                  roomCache,
		roomMemberCache:            roomMemberCache,
		txManager:                  txManager,
		idGenerator:                idGenerator,
	}
}

func (ra *RoomApplication) Create(ctx context.Context, userId, roomName, avatar, description string) (*RoomAppDTO, error) {
	roomId, err := ra.idGenerator.Generate()
	if err != nil {
		return nil, err
	}
	conversationId := conversationentity.GetConversationID(userId, roomId, int(conversationvo.RoomChat))

	room, err := roomentity.NewRoom(roomId, userId, description, roomName, avatar)
	if err != nil {
		if errors.Is(err, roomentity.ErrRoomNameRequired) {
			return nil, ErrRoomNameRequired
		} else {
			return nil, ErrUnknown
		}
	}
	roomUser := roomentity.NewRoomUser(userId, roomId, roomvo.HomeOwner)
	roomUser.Join()

	conversation := conversationentity.NewConversation(
		conversationId,
		userId,
		roomId,
		int(conversationvo.RoomChat),
		0,
		"",
	)

	userConversation := conversationentity.BuildUserConversation(
		userId,
		conversationId,
		0,
		0,
		conversationvo.RoomChat,
	)

	inviteCode, err := ra.roomCache.UpdateInviteCode(ctx, roomId, 5*time.Minute)
	if err != nil {
		log.Println("生成邀请码失败：", err)
		return nil, ErrInviteCodeUnavailable
	}

	if err := ra.txManager.WithinTransaction(ctx, func(tx any) error {
		roomRepository := ra.roomRepository.WithTx(tx)
		roomUserRepository := ra.roomUserRepository.WithTx(tx)
		conversationRepository := ra.conversationRepository.WithTx(tx)
		userConversationRepository := ra.userConversationRepository.WithTx(tx)

		if err := roomRepository.Create(room); err != nil {
			if errors.Is(err, roomentity.ErrDuplicateCreation) {
				return ErrConcurrentUpdate
			}
			return err
		}

		if _, err := roomUserRepository.JoinRoom(roomUser); err != nil {
			return err
		}

		if err := conversationRepository.CreateConversation(ctx, conversation); err != nil {
			return err
		}

		if err := userConversationRepository.CreateUserConversation(ctx, userConversation); err != nil {
			return err
		}

		return nil
	}); err != nil {
		if cleanupErr := ra.roomCache.DeleteInviteCode(ctx, roomId); cleanupErr != nil {
			log.Println("清理邀请码缓存失败：", cleanupErr)
		}
		return nil, err
	}

	memberState := &roomcache.MemberState{Status: roomUser.Status, Role: roomUser.Role, MuteUntil: roomUser.MuteUtil}
	if err := ra.roomMemberCache.SetMember(ctx, roomId, userId, memberState); err != nil {
		// best-effort cache warmup; the DB state is already authoritative
		log.Println("创建房间成员缓存失败：", err)
	}
	_ = ra.roomMemberCache.SetMemberIDs(ctx, roomId, []string{userId})

	return toRoomAppDTO(room, inviteCode), nil
}

func (ra *RoomApplication) Invite(ctx context.Context, userId, roomId string) (string, error) {
	room, err := ra.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate))
	if err != nil {
		if errors.Is(err, roomentity.ErrRoomNotFound) {
			return "", ErrRoomNotFound
		}

		log.Println("查询房间失败：", err)
		return "", ErrUnknown
	}

	roomUser, err := ra.roomUserRepository.GetRelationByIDs(userId, roomId)
	if err != nil {
		if errors.Is(err, roomentity.ErrMemberNotFound) {
			return "", ErrNotRoomMember
		}
		return "", ErrUnknown
	}

	if err := roomUser.Invite(); err != nil {
		return "", ErrPermissionDenied
	}

	if !room.Invite() {
		return "", ErrRoomUnavailable
	}

	inviteCode, err := ra.roomCache.UpdateInviteCode(ctx, roomId, 5*time.Minute)

	if err != nil {
		log.Println("获取邀请码失败：", err)
		return "", ErrInviteCodeUnavailable
	}

	return inviteCode, nil
}

func (ra *RoomApplication) Join(ctx context.Context, userId, inviteCode string) (*RoomUserDTO, *RoomAppDTO, error) {
	roomId, err := ra.roomCache.GetRoomIDByCode(ctx, inviteCode)
	conversationId := roomId
	if err != nil {
		if errors.Is(err, roomentity.ErrInviteCodeExpired) {
			return nil, nil, ErrInviteCodeExpired
		}

		log.Println("根据邀请码获取房间 ID 失败：", err)
		return nil, nil, ErrUnknown
	}

	room, err := ra.roomRepository.FindActiveRoom(roomId, int(roomvo.Normal))
	if err != nil {
		if errors.Is(err, roomentity.ErrRoomNotFound) {
			return nil, nil, ErrRoomNotFound
		}

		return nil, nil, ErrUnknown
	}

	roomUser := roomentity.NewRoomUser(userId, roomId, roomvo.RegularUser)
	roomUser.Join()

	if err := ra.txManager.WithinTransaction(ctx, func(tx any) error {
		roomUserRepository := ra.roomUserRepository.WithTx(tx)
		userConversationRepository := ra.userConversationRepository.WithTx(tx)
		conversationRepository := ra.conversationRepository.WithTx(tx)

		if _, err := roomUserRepository.JoinRoom(roomUser); err != nil {
			if !errors.Is(err, roomentity.ErrDuplicateCreation) {
				return err
			}
		}

		conv, err := conversationRepository.GetByID(ctx, conversationId)
		if err != nil {
			return err
		}

		userConversation := conversationentity.BuildUserConversation(
			userId,
			conversationId,
			conv.LatestSeq,
			conv.LatestSeq,
			conversationvo.RoomChat,
		)

		if err := userConversationRepository.CreateUserConversation(ctx, userConversation); err != nil {
			if errors.Is(err, roomentity.ErrVersionConflict) {
				return ErrConcurrentUpdate
			}
			return ErrUnknown
		}

		return nil
	}); err != nil {
		return nil, nil, err
	}

	memberState := &roomcache.MemberState{Status: roomUser.Status, Role: roomUser.Role, MuteUntil: roomUser.MuteUtil}
	if err := ra.roomMemberCache.SetMember(ctx, roomId, userId, memberState); err != nil {
		// best-effort cache warmup; the DB state is already authoritative
		log.Println("更新加入房间缓存失败：", err)
	}
	_ = ra.roomMemberCache.DeleteMemberIDs(ctx, roomId)

	return toRoomUserDTO(roomUser), toRoomAppDTO(room, ""), nil
}

func (ra *RoomApplication) Leave(ctx context.Context, userId, roomId string) error {
	roomUser, err := ra.roomUserRepository.GetRelationByIDs(userId, roomId)
	if err != nil {
		return ErrUnknown
	}

	if roomUser == nil {
		return ErrNotRoomMember
	}

	if err := roomUser.Leave(); err != nil {
		return err
	}

	if err := ra.txManager.WithinTransaction(ctx, func(tx any) error {
		roomUserRepo := ra.roomUserRepository.WithTx(tx)

		if err := roomUserRepo.LeaveRoom(roomUser, []int{int(roomvo.Activate), int(roomvo.BeMuted)}); err != nil {
			if errors.Is(err, roomentity.ErrVersionConflict) {
				return ErrConcurrentUpdate
			}
			return ErrUnknown
		}

		return nil
	}); err != nil {
		return err
	}

	if err := ra.roomMemberCache.SetMemberNotFound(ctx, roomId, userId); err != nil {
		// best-effort cache warmup; the DB state is already authoritative
		log.Println("更新退出房间缓存失败：", err)
	}
	_ = ra.roomMemberCache.DeleteMemberIDs(ctx, roomId)

	return nil
}
