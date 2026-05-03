package application_room

import (
	"IM_backend/configs"
	conversation_port "IM_backend/internal/application/ports/cache/conversation"
	room_cache_interface "IM_backend/internal/application/ports/cache/room"
	tx_repository_interface "IM_backend/internal/application/ports/persistence/tx_manager"
	message_repository_interface "IM_backend/internal/application/ports/repository/message"
	room_repository_interface "IM_backend/internal/application/ports/repository/room"
	message_entity "IM_backend/internal/domain/message/entity"
	message_valueobject "IM_backend/internal/domain/message/value_object"
	room_entity "IM_backend/internal/domain/room/entity"
	room_valueobject "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/id/snow"
	"IM_backend/internal/infrastructure/persistence/redis/cache/local"
	"context"
	"errors"
	"log"

	"gorm.io/gorm"
)

type RoomApplication struct {
	roomRepository             room_repository_interface.RoomRepository
	roomUserRepository         room_repository_interface.RoomUserRepository
	userConversationRepository message_repository_interface.UserConversationRepository
	conversationRepository     message_repository_interface.ConversationRepository
	config                     configs.Config
	conversationCache          conversation_port.ConversationCache
	localConvVersionCache      *local.ConversationVersionCache
	roomCache                  room_cache_interface.RoomCache
	txManager                  tx_repository_interface.TxManager
}

func NewRoomApplication(roomRepository room_repository_interface.RoomRepository,
	roomUserRepository room_repository_interface.RoomUserRepository,
	userConversationRepository message_repository_interface.UserConversationRepository,
	conversationRepository message_repository_interface.ConversationRepository,
	config configs.Config,
	roomCache room_cache_interface.RoomCache,
	conversationCache conversation_port.ConversationCache,
	localConvVersionCache *local.ConversationVersionCache,
	txManager tx_repository_interface.TxManager,
) *RoomApplication {
	return &RoomApplication{
		roomRepository:             roomRepository,
		roomUserRepository:         roomUserRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
		config:                     config,
		roomCache:                  roomCache,
		conversationCache:          conversationCache,
		localConvVersionCache:      localConvVersionCache,
		txManager:                  txManager,
	}
}

func (ra *RoomApplication) Create(ctx context.Context, userId, roomName, avatar, description string) (*RoomAppDTO, error) {
	roomId, err := snow.GenerateSnowID(int(ra.config.App.MachineID))
	conversationId := message_entity.GetConversationID(userId, roomId, int(message_valueobject.RoomChat))
	if err != nil {
		return nil, err
	}

	room, err := room_entity.NewRoom(roomId, userId, description, roomName, avatar)
	if err != nil {
		if errors.Is(err, room_entity.ErrRoomNameRequired) {
			return nil, ErrRoomNameRequired
		} else {
			return nil, ErrUnknown
		}
	}
	roomUser := room_entity.NewRoomUser(userId, roomId, room_valueobject.HomeOwner)
	roomUser.Join()

	conversation := message_entity.BuildConversation(
		conversationId,
		userId,
		roomId,
		int(message_valueobject.RoomChat),
		0,
		"",
	)

	userConversation := message_entity.BuildUserConversation(
		userId,
		conversationId,
		0,
		0,
	)

	if err := ra.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		roomRepository := ra.roomRepository.WithTx(tx)
		roomUserRepository := ra.roomUserRepository.WithTx(tx)
		conversationRepository := ra.conversationRepository.WithTx(tx)
		userConversationRepository := ra.userConversationRepository.WithTx(tx)

		if err := roomRepository.Create(room); err != nil {
			if errors.Is(err, room_entity.ErrDuplicateCreation) {
				return ErrConcurrentUpdate
			}
			return err
		}

		if _, err := roomUserRepository.Create(roomUser); err != nil {
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
		return nil, err
	}

	// 创建房间，会话成员初始化是 1
	ra.localConvVersionCache.SetVersion(conversationId, 1)
	inviteCode, inviteErr := ra.roomCache.UpdateInviteCode(ctx, roomId, 5)

	if err := ra.conversationCache.AddMember(ctx, conversationId, userId, 1); err != nil {
		// TODO: 异步补偿
		log.Println("failed to create join room cache ", err)
	}

	if inviteErr != nil {
		log.Println("failed to generate invite code:", inviteErr)
		inviteCode = "110234675"
	}

	return toRoomAppDTO(room, inviteCode), nil
}

func (ra *RoomApplication) Invite(ctx context.Context, userId, roomId string) (string, error) {
	room, err := ra.roomRepository.FindActiveRoom(roomId, int(room_valueobject.Activate))
	if err != nil {
		if errors.Is(err, room_entity.ErrRoomNotFound) {
			return "", ErrRoomNotFound
		}

		log.Println("failed to get room ", err)
		return "", ErrUnknown
	}

	roomUser, err := ra.roomUserRepository.GetRelationByIDs(userId, roomId)
	if err != nil {
		if errors.Is(err, room_entity.ErrMemberNotFound) {
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

	invitecode, err := ra.roomCache.UpdateInviteCode(ctx, roomId, 5)

	if err != nil {
		log.Println("failed to get invite code, ", err)
		return "", nil
	}

	return invitecode, nil
}

func (ra *RoomApplication) Join(ctx context.Context, userId, inviteCode string) (*RoomUserDTO, *RoomAppDTO, error) {
	roomId, err := ra.roomCache.GetRoomIDByCode(ctx, inviteCode)
	conversationId := roomId
	if err != nil {
		if errors.Is(err, room_entity.ErrInviteCodeExpired) {
			return nil, nil, ErrInviteCodeExpired
		}

		log.Println("failed to get roomId ", err)
		return nil, nil, ErrUnknown
	}

	room, err := ra.roomRepository.FindActiveRoom(roomId, int(room_valueobject.Normal))
	if err != nil {
		if errors.Is(err, room_entity.ErrRoomNotFound) {
			return nil, nil, ErrRoomNotFound
		}

		return nil, nil, ErrUnknown
	}

	roomUser := room_entity.NewRoomUser(userId, roomId, room_valueobject.RegularUser)
	roomUser.Join()

	var version int64
	if err := ra.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		roomUserRepository := ra.roomUserRepository.WithTx(tx)
		userConversationRepository := ra.userConversationRepository.WithTx(tx)
		conversationRepository := ra.conversationRepository.WithTx(tx)

		if err := roomUserRepository.JoinRoom(roomUser); err != nil {
			if !errors.Is(err, room_entity.ErrDuplicateJoin) {
				return err
			}
		}

		conv, err := conversationRepository.GetByID(ctx, conversationId)
		if err != nil {
			return err
		}

		if version, err = ra.roomRepository.UpdateRoomVersion(roomId); err != nil {
			return err
		}

		userConversation := message_entity.BuildUserConversation(
			userId,
			conversationId,
			conv.LatestSeq,
			conv.LatestSeq,
		)

		if err := userConversationRepository.CreateUserConversation(ctx, userConversation); err != nil {
			if errors.Is(err, room_entity.ErrVersionConflict) {
				return ErrConcurrentUpdate
			}
			return ErrUnknown
		}

		return nil
	}); err != nil {
		return nil, nil, err
	}

	ra.localConvVersionCache.SetVersion(conversationId, version)
	if err := ra.conversationCache.AddMember(ctx, conversationId, userId, version); err != nil {
		// TODO: 异步补偿
		log.Println("failed to update join room cache ", err)
	}

	return toRoomUserDTO(roomUser), toRoomAppDTO(room, ""), nil
}

func (ra *RoomApplication) Leave(ctx context.Context, userId, roomId string) error {
	roomUser, err := ra.roomUserRepository.GetRelationByIDs(userId, roomId)
	conversationId := roomId
	if err != nil {
		return ErrUnknown
	}

	if roomUser == nil {
		return ErrNotRoomMember
	}

	if err := roomUser.Leave(); err != nil {
		return err
	}

	var version int64
	if err := ra.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		roomUserRepository := ra.roomUserRepository.WithTx(tx)
		if err := roomUserRepository.JoinRoom(roomUser); err != nil {
			if !errors.Is(err, room_entity.ErrDuplicateJoin) {
				return err
			}
		}

		if version, err = ra.roomRepository.UpdateRoomVersion(roomId); err != nil {
			return err
		}

		if err := ra.roomUserRepository.Leave(roomUser, []int{int(room_valueobject.Activate), int(room_valueobject.BeMuted)}); err != nil {
			if errors.Is(err, room_entity.ErrVersionConflict) {
				return ErrConcurrentUpdate
			}
			return ErrUnknown
		}

		return nil
	}); err != nil {
		return err
	}

	ra.localConvVersionCache.SetVersion(conversationId, version)
	if err := ra.conversationCache.RemoveMember(ctx, conversationId, userId, version); err != nil {
		// TODO: 异步补偿
		log.Println("failed to update remove room cache ", err)
	}

	return nil
}
