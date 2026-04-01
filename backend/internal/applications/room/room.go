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
	roomId, err := snow.GenerateSnowId(int(ra.config.Snowflake.MachineID))

	if err != nil {
		return nil, err
	}

	roomDomain, err := room_entity.NewRoom(roomId, userId, description, roomName, avatar)
	if err != nil {
		if errors.Is(err, room_entity.ErrRoomNameIsNotNull) {
			return nil, ErrRoomNameIsNotNull
		} else {
			return nil, ErrUnknownError
		}
	}
	roomUserDomain := room_entity.NewRoomUser(userId, roomId, room_valueobject.HomeOwner)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(3)*time.Second)
	defer cancel()

	if err := ra.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		roomRepository := ra.roomRepository.WithTx(tx)
		roomUserRepository := ra.roomUserRepository.WithTx(tx)

		if _, err := roomRepository.Create(roomDomain); err != nil {
			return err
		}

		if _, err := roomUserRepository.Create(roomUserDomain); err != nil {
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

	return toDTO(roomDomain, inviteCode), nil
}

func (ra *RoomApplication) Invite(ctx context.Context, userId, roomId string) (string, error) {
	room, err := ra.roomRepository.FindRoomByRoomId(roomId)
	if err != nil {
		if errors.Is(err, room_entity.ErrRoomNotFound) {
			return "", ErrRoomNotFound
		}

		log.Println("failed to get room ", err)
		return "", ErrUnknownError
	}

	if !room.CanInvite() {
		return "", ErrNotAvaiableRoom
	}

	invitecode, err := ra.roomCache.UpdateInviteCode(ctx, roomId, 5)

	if err != nil {
		log.Println("failed to get invite code, ", err)
		return "", nil
	}

	return invitecode, nil
}

func (ra *RoomApplication) Join(ctx context.Context, userId, roomId string) (string, error)
