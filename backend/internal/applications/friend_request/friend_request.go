package application_friend_request

import (
	"IM_backend/configs"
	application_friend "IM_backend/internal/applications/friend"
	tx_repository_interface "IM_backend/internal/applications/interface/repository/tx_manager"
	friend_entity "IM_backend/internal/domain/friend/entity"
	friend_repository_interface "IM_backend/internal/domain/friend/repository"
	friend_valueobject "IM_backend/internal/domain/friend/value_object"
	friend_request_entity "IM_backend/internal/domain/frient_request/entity"
	friend_request_repository_interface "IM_backend/internal/domain/frient_request/repository"
	friend_request_valueobject "IM_backend/internal/domain/frient_request/value_object"
	user_repository_interface "IM_backend/internal/domain/user/repository"
	"IM_backend/internal/infrastructure/pkg/snow"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

type FriendApplication struct {
	friendRequestRepository friend_request_repository_interface.FriendRequestInterface
	userRepository          user_repository_interface.UserRepoInterface
	friendRepository        friend_repository_interface.FriendRepositoryInterface
	config                  configs.Config
	txManager               tx_repository_interface.TxRepositoryInterface
}

func NewFriendApplication(
	friendRequestRepository friend_request_repository_interface.FriendRequestInterface,
	userRepository user_repository_interface.UserRepoInterface,
	config configs.Config,
	friendRepository friend_repository_interface.FriendRepositoryInterface,
	txManager tx_repository_interface.TxRepositoryInterface,
) *FriendApplication {
	return &FriendApplication{
		friendRequestRepository: friendRequestRepository,
		userRepository:          userRepository,
		config:                  config,
		friendRepository:        friendRepository,
		txManager:               txManager,
	}
}

func (fa *FriendApplication) NewFriendRequest(userId, toUserId, message string) (FriendRequestDTO, error) {
	// TODO 后期优化结构
	user, err := fa.userRepository.FindByUserId(toUserId)
	if err != nil {
		return FriendRequestDTO{}, err
	}

	if user == nil {
		return FriendRequestDTO{}, errors.New("用户不存在")
	}

	record, err := fa.friendRequestRepository.FindLatestRequest(userId, toUserId)
	if err != nil {
		return FriendRequestDTO{}, err
	}

	if record == nil {
		requestId, err := snow.GenerateSnowId(int(fa.config.Snowflake.MachineID))
		if err != nil {
			return FriendRequestDTO{}, err
		}
		// 建立新的申请对象
		newFriendRequest, err := friend_request_entity.NewFriendRequest(requestId, userId, toUserId, message)
		if err != nil {
			if errors.Is(err, friend_request_entity.ErrRequestSelf) {
				return FriendRequestDTO{}, ErrRequestSelf
			} else {
				return FriendRequestDTO{}, ErrUnknown
			}
		}

		// 入库
		newRecord, err := fa.friendRequestRepository.Create(newFriendRequest)
		if err != nil {
			return FriendRequestDTO{}, err
		}

		return toDTO(newRecord), nil
	} else {
		relation, err := fa.friendRepository.FindRelation(userId, toUserId)

		if err != nil {
			return FriendRequestDTO{}, err
		}

		if relation != nil {
			return FriendRequestDTO{}, application_friend.ErrAlreadyFriend
		}

		switch record.Status {
		case friend_request_valueobject.Pending:
			if err := record.ReRequest(message); err != nil {
				if errors.Is(friend_request_entity.ErrApplyingTooFrequently, err) {
					return FriendRequestDTO{}, ErrApplyingTooFrequently
				} else {
					return FriendRequestDTO{}, err
				}
			}

			if err := fa.friendRequestRepository.ReRequest(record); err != nil {
				return FriendRequestDTO{}, err
			}

			return toDTO(record), nil
		default:
			// 重新创建记录，状态转换已经结束 （Accept / Refused）
			if err := record.ReRequest(message); err != nil {
				switch err {
				case friend_request_entity.ErrApplyingTooFrequently:
					return FriendRequestDTO{}, ErrApplyingTooFrequently
				case friend_request_entity.ErrInvalidateStatus:
					return FriendRequestDTO{}, ErrInvalidStatusTransition
				case friend_request_entity.ErrRequestSelf:
					return FriendRequestDTO{}, ErrRequestSelf
				default:
					return FriendRequestDTO{}, ErrUnknown
				}
			}

			newRecord, err := fa.friendRequestRepository.Create(record)
			if err != nil {
				return FriendRequestDTO{}, ErrUnknown
			}

			return toDTO(newRecord), nil
		}
	}
}

func (fa *FriendApplication) Refuse(requestId string, userId string) error {
	record, err := fa.friendRequestRepository.FindByRequestId(requestId)

	if err != nil {
		return err
	}

	if err := record.Refuse(userId); err != nil {
		return err
	}

	if err := fa.friendRequestRepository.OperateRequest(record.RequestId, int(friend_request_valueobject.Pending), int(record.Status)); err != nil {
		return err
	}

	return nil
}

func (fa *FriendApplication) Accept(requestId string, userId string) error {
	record, err := fa.friendRequestRepository.FindByRequestId(requestId)

	if err != nil {
		return err
	}

	if err := record.Accept(userId); err != nil {
		if errors.Is(err, friend_request_entity.ErrDuplicateOperation) {
			return ErrDuplicateOperation
		} else {
			return ErrUnknown
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := fa.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		friendRepository := fa.friendRepository.WithTx(tx)
		friendRequestRepository := fa.friendRequestRepository.WithTx(tx)

		if err := friendRequestRepository.OperateRequest(record.RequestId, int(friend_request_valueobject.Pending), int(record.Status)); err != nil {
			fmt.Println("failed to operate request, ", err)
			return ErrOperateFailed
		}

		// 创建好友
		if err := friendRepository.Create([]friend_entity.Friend{
			{
				UserId:       record.FromUserId,
				FriendUserId: record.ToUserId,
				Status:       friend_valueobject.Status(friend_valueobject.Friend),
			},
			{
				UserId:       record.ToUserId,
				FriendUserId: record.FromUserId,
				Status:       friend_valueobject.Status(friend_valueobject.Friend),
			},
		}); err != nil {
			log.Println("failed to create friends, ", err)
			return ErrOperateFailed
		}
		return nil
	}); err != nil {
		return ErrOperateFailed
	}

	return nil
}

func (fa *FriendApplication) GetFriendRequstListByUserId(userId string) ([]FriendRequestDTO, error) {
	records, err := fa.friendRequestRepository.ListByUserId(userId)
	if err != nil {
		return nil, err
	}

	friendRequestDTOs := make([]FriendRequestDTO, 0, len(records))
	for _, r := range records {
		friendRequestDTOs = append(friendRequestDTOs, toDTO(r))
	}

	return friendRequestDTOs, nil
}
