package application_room

import (
	"IM_backend/configs"
	message_cache_interface "IM_backend/internal/applications/interface/cache/message"
	room_cache_interface "IM_backend/internal/applications/interface/cache/room"
	tx_repository_interface "IM_backend/internal/applications/interface/repository/tx_manager"
	message_entity "IM_backend/internal/domain/message/entity"
	message_repository_interface "IM_backend/internal/domain/message/repository"
	message_valueobject "IM_backend/internal/domain/message/value_object"
	room_entity "IM_backend/internal/domain/room/entity"
	room_repository_interface "IM_backend/internal/domain/room/repository"
	room_valueobject "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/pkg/snow"
	conversation_port "IM_backend/internal/port/conversation"
	"context"
	"errors"
	"log"

	"gorm.io/gorm"
)

type RoomApplication struct {
	roomRepository             room_repository_interface.RoomRepositoryInterface
	roomUserRepository         room_repository_interface.RoomUserRepositoryInterface
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface
	conversationRepository     message_repository_interface.ConversationRepositoryInterface
	config                     configs.Config
	conversationCache          conversation_port.ConversationCacheInterface
	roomCache                  room_cache_interface.RoomCacheInterface
	messageConversation        message_cache_interface.MessageCacheInterface
	txManager                  tx_repository_interface.TxRepositoryInterface
}

func NewRoomApplication(roomRepository room_repository_interface.RoomRepositoryInterface,
	roomUserRepository room_repository_interface.RoomUserRepositoryInterface,
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface,
	conversationRepository message_repository_interface.ConversationRepositoryInterface,
	config configs.Config,
	roomCache room_cache_interface.RoomCacheInterface,
	messageConversation message_cache_interface.MessageCacheInterface,
	conversationCache conversation_port.ConversationCacheInterface,
	txManager tx_repository_interface.TxRepositoryInterface,
) *RoomApplication {
	return &RoomApplication{
		roomRepository:             roomRepository,
		roomUserRepository:         roomUserRepository,
		conversationRepository:     conversationRepository,
		userConversationRepository: userConversationRepository,
		config:                     config,
		roomCache:                  roomCache,
		messageConversation:        messageConversation,
		conversationCache:          conversationCache,
		txManager:                  txManager,
	}
}

func (ra *RoomApplication) Create(ctx context.Context, userId, roomName, avatar, description string) (*RoomAppDTO, error) {
	roomId, err := snow.GenerateSnowId(int(ra.config.App.MachineID))
	conversationId := roomId
	if err != nil {
		return nil, err
	}

	room, err := room_entity.NewRoom(roomId, userId, description, roomName, avatar)
	if err != nil {
		if errors.Is(err, room_entity.ErrRoomNameIsNotNull) {
			return nil, ErrRoomNameIsNotNull
		} else {
			return nil, ErrUnknownError
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
	)

	userConversation := message_entity.BuildUserConversation(
		userId,
		conversationId,
		"",
		0,
		0,
	)

	if err := ra.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		roomRepository := ra.roomRepository.WithTx(tx)
		roomUserRepository := ra.roomUserRepository.WithTx(tx)
		conversationRepository := ra.conversationRepository.WithTx(tx)
		userConversationRepository := ra.userConversationRepository.WithTx(tx)

		if _, err := roomRepository.Create(room); err != nil {
			if errors.Is(err, room_entity.ErrDuplicateCreate) {
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

	inviteCode, inviteErr := ra.roomCache.UpdateInviteCode(ctx, roomId, 5)

	if err := ra.conversationCache.AddMember(ctx, conversationId, userId); err != nil {
		log.Println("add member cache failed:", err)
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
		return "", ErrUnknownError
	}

	// 查看当前用户是否在房间内
	roomUser, err := ra.roomUserRepository.GetRelationByIds(userId, roomId)
	if err != nil {
		if errors.Is(err, room_entity.ErrRecordNotFound) {
			return "", ErrNotInRoom
		}
		return "", ErrUnknownError
	}

	if err := roomUser.Invite(); err != nil {
		return "", ErrNoPermission
	}

	if !room.Invite() {
		return "", ErrNotAvaiableRoom
	}

	invitecode, err := ra.roomCache.UpdateInviteCode(ctx, roomId, 5)

	if err != nil {
		log.Println("failed to get invite code, ", err)
		return "", nil
	}

	return invitecode, nil
}

func (ra *RoomApplication) Join(ctx context.Context, userId, inviteCode string) (*RoomUserDTO, *RoomAppDTO, error) {
	roomId, err := ra.roomCache.GetRoomIdByCode(ctx, inviteCode)
	conversationId := roomId
	if err != nil {
		if errors.Is(err, room_entity.ErrUnavaiableCode) {
			return nil, nil, ErrUnavaiableCode
		}

		log.Println("failed to get roomId ", err)
		return nil, nil, ErrUnknownError
	}

	room, err := ra.roomRepository.FindActiveRoom(roomId, int(room_valueobject.Normal))
	if err != nil {
		if errors.Is(err, room_entity.ErrRoomNotFound) {
			return nil, nil, ErrRoomNotFound
		}

		return nil, nil, ErrUnknownError
	}

	roomUser := room_entity.NewRoomUser(userId, roomId, room_valueobject.RegularUser)
	roomUser.Join()

	if err := ra.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		roomUserRepository := ra.roomUserRepository.WithTx(tx)
		userConversationRepository := ra.userConversationRepository.WithTx(tx)
		conversationRepository := ra.conversationRepository.WithTx(tx)

		if err := roomUserRepository.JoinRoom(roomUser); err != nil {
			if !errors.Is(err, room_entity.ErrDuplicateJoin) {
				return err
			}
		}

		curMessageSeq, err := conversationRepository.GetConvSeq(ctx, conversationId)
		if err != nil {
			return err
		}

		userConversation := message_entity.BuildUserConversation(
			userId,
			conversationId,
			"",
			curMessageSeq,
			curMessageSeq,
		)

		if err := userConversationRepository.CreateUserConversation(ctx, userConversation); err != nil {
			if !errors.Is(err, message_entity.ErrDuplicateCreate) {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, nil, err
	}

	if err := ra.conversationCache.AddMember(ctx, conversationId, userId); err != nil {
		log.Println("add member cache failed:", err)

		// TODO: 交给 mq 去重新加入
	}

	return toRoomUserDTO(roomUser), toRoomAppDTO(room, ""), nil
}

func (ra *RoomApplication) Leave(ctx context.Context, userId, roomId string) error {
	roomUser, err := ra.roomUserRepository.GetRelationByIds(userId, roomId)
	conversationId := roomId
	if err != nil {
		return ErrUnknownError
	}

	if roomUser == nil {
		return ErrNotInRoom
	}

	if err := roomUser.Leave(); err != nil {
		return err
	}

	if err := ra.roomUserRepository.Leave(roomUser, []int{int(room_valueobject.Activate), int(room_valueobject.BeMuted)}); err != nil {
		if errors.Is(err, room_entity.ErrVersionConflict) {
			return ErrConcurrentUpdate
		}
		return ErrUnknownError
	}

	if err := ra.conversationCache.RemoveMember(ctx, conversationId, userId); err != nil {
		return ErrConnectRoom
	}

	return nil
}
