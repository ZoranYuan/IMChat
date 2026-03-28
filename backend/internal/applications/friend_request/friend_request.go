package application_friend_request

import (
	"IM_backend/configs"
	friend_request "IM_backend/internal/domain/frient_request"
	friend_request_entity "IM_backend/internal/domain/frient_request/entity"
	friend_request_valueobject "IM_backend/internal/domain/frient_request/value_object"
	"IM_backend/internal/domain/user"
	"IM_backend/internal/infrastructure/pkg/snow"
	"errors"
)

type FriendApplication struct {
	friendRequestRepository friend_request.FriendRequestInterface
	userRepository          user.UserRepoInterface
	config                  configs.Config
}

func NewFriendApplication(friendRequestRepository friend_request.FriendRequestInterface, userRepository user.UserRepoInterface) *FriendApplication {
	return &FriendApplication{
		friendRequestRepository: friendRequestRepository,
		userRepository:          userRepository,
	}
}

func (fa *FriendApplication) NewFriendRequest(userId, toUserId, message string) (*FriendRequestDTO, error) {
	user, err := fa.userRepository.FindUserByUserId(toUserId)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("用户不存在")
	}

	record, err := fa.friendRequestRepository.FindByUsersByIds(userId, toUserId)
	if err != nil {
		return nil, err
	}

	if record == nil {
		requestId, err := snow.GenerateSnowId(int(fa.config.Snowflake.MachineID))
		if err != nil {
			return nil, err
		}
		// 建立新的申请对象
		newFriendRequest := friend_request_entity.NewFriendRequest(requestId, userId, toUserId, message)

		// 入库
		newRecord, err := fa.friendRequestRepository.Create(newFriendRequest)
		if err != nil {
			return nil, err
		}

		return toDTO(newRecord), nil
	} else {
		// 这里需要排除二者已经是好友了

		switch record.Status {
		case friend_request_valueobject.Accepted:
			// TODO 重新创建一个新的请求
		default:
			if err := record.ReRequest(message); err != nil {
				// record.ReRequest(message) 只会返回 ErrApplyingTooFrequently 错误，app 层做映射投回 api ，控制数据流向
				return nil, friend_request_entity.ErrApplyingTooFrequently
			}
			if err := fa.friendRequestRepository.ReRequest(record,
				[]int{int(friend_request_valueobject.Pending), int(friend_request_valueobject.Refused)},
			); err != nil {
				return nil, err
			}
		}
		return toDTO(record), nil
	}
}

func (fa *FriendApplication) Refuse(requestId string) error {
	record, err := fa.friendRequestRepository.FindByRequestId(requestId)

	if err != nil {
		return err
	}

	if err := record.Refuse(); err != nil {
		return err
	}

	if err := fa.friendRequestRepository.OperateRequest(record.RequestId, int(friend_request_valueobject.Pending), int(record.Status)); err != nil {
		return err
	}

	return nil
}

func (fa *FriendApplication) Accept(requestId string) error {
	record, err := fa.friendRequestRepository.FindByRequestId(requestId)

	if err != nil {
		return err
	}

	if err := record.Accept(); err != nil {
		return err
	}

	if err := fa.friendRequestRepository.OperateRequest(record.RequestId, int(friend_request_valueobject.Pending), int(record.Status)); err != nil {
		return err
	}

	return nil
}

func (fa *FriendApplication) GetFriendRequstListByUserId(userId string) ([]*FriendRequestDTO, error) {
	records, err := fa.friendRequestRepository.ListByUserId(userId)
	if err != nil {
		return nil, err
	}

	friendRequestDTOs := make([]*FriendRequestDTO, 0, len(records))
	for _, r := range records {
		friendRequestDTOs = append(friendRequestDTOs, toDTO(r))
	}

	return friendRequestDTOs, nil
}
