package application_room

import (
	"IM_backend/configs"
	room_cache_interface "IM_backend/internal/applications/interface/cache/room"
	tx_repository_interface "IM_backend/internal/applications/interface/repository/tx_manager"
	room_entity "IM_backend/internal/domain/room/entity"
	room_repository_interface "IM_backend/internal/domain/room/repository"
	room_valueobject "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/pkg/snow"
	"context"
	"errors"
	"log"
	"time"

	"gorm.io/gorm"
)

type RoomApplication struct {
	roomRepository     room_repository_interface.RoomRepositoryInterface
	roomUserRepository room_repository_interface.RoomUserRepositoryInterface
	config             configs.Config
	roomCache          room_cache_interface.RoomCacheInterface
	txManager          tx_repository_interface.TxRepositoryInterface
}

func NewRoomApplication(roomRepository room_repository_interface.RoomRepositoryInterface,
	roomUserRepository room_repository_interface.RoomUserRepositoryInterface,
	config configs.Config,
	roomCache room_cache_interface.RoomCacheInterface,
	txManager tx_repository_interface.TxRepositoryInterface,
) *RoomApplication {
	return &RoomApplication{
		roomRepository:     roomRepository,
		roomUserRepository: roomUserRepository,
		config:             config,
		roomCache:          roomCache,
		txManager:          txManager,
	}
}

func (ra *RoomApplication) Create(ctx context.Context, userId, roomName, avatar, description string) (*RoomAppDTO, error) {
	roomId, err := snow.GenerateSnowId(int(ra.config.App.MachineID))

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

	ctx, cancel := context.WithTimeout(ctx, time.Duration(3)*time.Second)
	defer cancel()

	if err := ra.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		roomRepository := ra.roomRepository.WithTx(tx)
		roomUserRepository := ra.roomUserRepository.WithTx(tx)

		if _, err := roomRepository.Create(room); err != nil {
			if errors.Is(err, room_entity.ErrDuplicateCreate) {
				return ErrConcurrentUpdate
			}
		}

		if _, err := roomUserRepository.Create(roomUser); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	inviteCode, err := ra.roomCache.UpdateInviteCode(ctx, roomId, 5)

	if err != nil {
		// 降级处理
		log.Println("failed to generate invite code ", err)
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
	roomUser, err := ra.roomUserRepository.GetRelationByIds(roomId, userId)
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

	if err != nil {
		if errors.Is(err, room_entity.ErrUnavaiableCode) {
			return nil, nil, ErrUnavaiableCode
		}

		log.Println("failed to get roomId ", err)
		return nil, nil, ErrUnknownError
	}

	room, err := ra.roomRepository.FindActiveRoom(roomId, int(room_valueobject.Normal))
	if err != nil {
		if errors.Is(err, room_entity.ErrUnavaiableCode) {
			return nil, nil, ErrUnavaiableCode
		}

		return nil, nil, ErrUnknownError
	}

	roomUser := room_entity.NewRoomUser(userId, roomId, room_valueobject.RegularUser)
	roomUser.Join()

	if err := ra.roomUserRepository.JoinRoom(roomUser); err != nil {
		if errors.Is(err, room_entity.ErrVersionConflict) {
			return nil, nil, ErrConcurrentUpdate
		}

		log.Println("failed to join the room, ", err)
		return nil, nil, ErrUnknownError
	}

	return toRoomUserDTO(roomUser), toRoomAppDTO(room, ""), nil
}

func (ra *RoomApplication) Leave(ctx context.Context, userId, roomId string) error {
	roomUser, err := ra.roomUserRepository.GetRelationByIds(userId, roomId)

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

	return nil
}
