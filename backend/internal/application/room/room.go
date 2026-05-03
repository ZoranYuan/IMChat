package room

import (
	"IM_backend/configs"
	convcache "IM_backend/internal/application/ports/persistence/cache/conversation"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/id/snow"
	"IM_backend/internal/infrastructure/persistence/redis/cache/local"
	"context"
	"errors"
	"log"

	"gorm.io/gorm"
)

type RoomApplication struct {
	roomRepository             roomrepo.RoomRepository
	roomUserRepository         roomrepo.RoomUserRepository
	userConversationRepository messagerepo.UserConversationRepository
	conversationRepository     messagerepo.ConversationRepository
	config                     configs.Config
	conversationCache          convcache.ConversationCache
	localConvVersionCache      *local.ConversationVersionCache
	roomCache                  roomcache.RoomCache
	txManager                  txmanager.TxManager
}

func NewRoomApplication(roomRepository roomrepo.RoomRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	userConversationRepository messagerepo.UserConversationRepository,
	conversationRepository messagerepo.ConversationRepository,
	config configs.Config,
	roomCache roomcache.RoomCache,
	conversationCache convcache.ConversationCache,
	localConvVersionCache *local.ConversationVersionCache,
	txManager txmanager.TxManager,
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
	conversationId := messageentity.GetConversationID(userId, roomId, int(messagevo.RoomChat))
	if err != nil {
		return nil, err
	}

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

	conversation := messageentity.BuildConversation(
		conversationId,
		userId,
		roomId,
		int(messagevo.RoomChat),
		0,
		"",
	)

	userConversation := messageentity.BuildUserConversation(
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
			if errors.Is(err, roomentity.ErrDuplicateCreation) {
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
	room, err := ra.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate))
	if err != nil {
		if errors.Is(err, roomentity.ErrRoomNotFound) {
			return "", ErrRoomNotFound
		}

		log.Println("failed to get room ", err)
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
		if errors.Is(err, roomentity.ErrInviteCodeExpired) {
			return nil, nil, ErrInviteCodeExpired
		}

		log.Println("failed to get roomId ", err)
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

	var version int64
	if err := ra.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		roomUserRepository := ra.roomUserRepository.WithTx(tx)
		userConversationRepository := ra.userConversationRepository.WithTx(tx)
		conversationRepository := ra.conversationRepository.WithTx(tx)

		if err := roomUserRepository.JoinRoom(roomUser); err != nil {
			if !errors.Is(err, roomentity.ErrDuplicateJoin) {
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

		userConversation := messageentity.BuildUserConversation(
			userId,
			conversationId,
			conv.LatestSeq,
			conv.LatestSeq,
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
			if !errors.Is(err, roomentity.ErrDuplicateJoin) {
				return err
			}
		}

		if version, err = ra.roomRepository.UpdateRoomVersion(roomId); err != nil {
			return err
		}

		if err := ra.roomUserRepository.Leave(roomUser, []int{int(roomvo.Activate), int(roomvo.BeMuted)}); err != nil {
			if errors.Is(err, roomentity.ErrVersionConflict) {
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
