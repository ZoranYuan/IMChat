package friendrequest

import (
	"IM_backend/configs"
	friendapp "IM_backend/internal/application/friend"
	convcache "IM_backend/internal/application/ports/persistence/cache/conversation"
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	friendrequestrepo "IM_backend/internal/application/ports/persistence/repository/friend_request"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	friendentity "IM_backend/internal/domain/friend/entity"
	friendvo "IM_backend/internal/domain/friend/value_object"
	friendrequestentity "IM_backend/internal/domain/friend_request/entity"
	friendrequestvo "IM_backend/internal/domain/friend_request/value_object"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	"IM_backend/internal/infrastructure/id/snow"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

type FriendApplication struct {
	friendRequestRepository friendrequestrepo.FriendRequestRepository
	userRepository          userrepo.UserRepository
	friendRepository        friendrepo.FriendRepository
	conversationCache       convcache.ConversationCache
	config                  configs.Config
	txManager               txmanager.TxManager
}

func NewFriendApplication(
	friendRequestRepository friendrequestrepo.FriendRequestRepository,
	userRepository userrepo.UserRepository,
	config configs.Config,
	friendRepository friendrepo.FriendRepository,
	conversationCache convcache.ConversationCache,
	txManager txmanager.TxManager,
) *FriendApplication {
	return &FriendApplication{
		friendRequestRepository: friendRequestRepository,
		userRepository:          userRepository,
		config:                  config,
		friendRepository:        friendRepository,
		conversationCache:       conversationCache,
		txManager:               txManager,
	}
}

func (fa *FriendApplication) CreateFriendRequest(userId, toUserId, message string) (FriendRequestDTO, error) {
	// TODO 后期优化结构
	user, err := fa.userRepository.FindByUserID(toUserId)
	if err != nil {
		return FriendRequestDTO{}, err
	}

	if user == nil {
		return FriendRequestDTO{}, ErrUserNotFound
	}

	record, err := fa.friendRequestRepository.FindLatestRequest(userId, toUserId)
	if err != nil {
		return FriendRequestDTO{}, err
	}

	if record == nil {
		requestId, err := snow.GenerateSnowID(int(fa.config.App.MachineID))
		if err != nil {
			return FriendRequestDTO{}, err
		}
		// 建立新的申请对象
		newFriendRequest, err := friendrequestentity.NewFriendRequest(requestId, userId, toUserId, message)
		if err != nil {
			if errors.Is(err, friendrequestentity.ErrSelfRequest) {
				return FriendRequestDTO{}, ErrSelfRequest
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
			return FriendRequestDTO{}, friendapp.ErrAlreadyFriends
		}

		switch record.Status {
		case friendrequestvo.Pending:
			if err := record.ReRequest(message); err != nil {
				if errors.Is(friendrequestentity.ErrRequestSentTooFrequently, err) {
					return FriendRequestDTO{}, ErrRequestSentTooFrequently
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
				case friendrequestentity.ErrRequestSentTooFrequently:
					return FriendRequestDTO{}, ErrRequestSentTooFrequently
				case friendrequestentity.ErrInvalidStatus:
					return FriendRequestDTO{}, ErrInvalidStatusTransition
				case friendrequestentity.ErrSelfRequest:
					return FriendRequestDTO{}, ErrSelfRequest
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
	record, err := fa.friendRequestRepository.FindByRequestID(requestId)

	if err != nil {
		return err
	}

	if err := record.Refuse(userId); err != nil {
		return err
	}

	if err := fa.friendRequestRepository.OperateRequest(record.RequestId, int(friendrequestvo.Pending), int(record.Status)); err != nil {
		return err
	}

	return nil
}

func (fa *FriendApplication) Accept(requestId string, userId string) error {
	record, err := fa.friendRequestRepository.FindByRequestID(requestId)

	if err != nil {
		return err
	}

	if err := record.Accept(userId); err != nil {
		if errors.Is(err, friendrequestentity.ErrDuplicateOperation) {
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

		if err := friendRequestRepository.OperateRequest(record.RequestId, int(friendrequestvo.Pending), int(record.Status)); err != nil {
			fmt.Println("failed to operate request, ", err)
			return ErrOperationFailed
		}

		if err := friendRepository.Create([]friendentity.Friend{
			{
				UserId:       record.FromUserId,
				FriendUserId: record.ToUserId,
				Status:       friendvo.Status(friendvo.Friend),
			},
			{
				UserId:       record.ToUserId,
				FriendUserId: record.FromUserId,
				Status:       friendvo.Status(friendvo.Friend),
			},
		}); err != nil {
			log.Println("failed to create friends, ", err)
			return ErrOperationFailed
		}
		return nil
	}); err != nil {
		return ErrOperationFailed
	}

	conversationId := messageentity.GetConversationID(record.ToUserId, record.FromUserId, int(messagevo.PrivateChat))
	if err := fa.conversationCache.SetMembers(ctx, conversationId, []string{record.FromUserId, record.ToUserId}, 1); err != nil {
		// TODO: 异步补偿
		log.Println("failed to create conversation cache")
	}

	return nil
}

func (fa *FriendApplication) ListFriendRequestsByUserID(userId string) ([]FriendRequestDTO, error) {
	records, err := fa.friendRequestRepository.ListByUserID(userId)
	if err != nil {
		return nil, err
	}

	friendRequestDTOs := make([]FriendRequestDTO, 0, len(records))
	for _, r := range records {
		friendRequestDTOs = append(friendRequestDTOs, toDTO(r))
	}

	return friendRequestDTOs, nil
}
