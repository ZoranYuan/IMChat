package application_friend

import (
	friend_request "IM_backend/internal/domain/frient_request"
	friend_request_entity "IM_backend/internal/domain/frient_request/entity"
	"IM_backend/internal/domain/user"
	"errors"
)

type FriendApplication struct {
	friendRequestRepository friend_request.FriendRequestInterface
	userRepository          user.UserRepoInterface
}

func NewFriendApplication(friendRequestRepository friend_request.FriendRequestInterface, userRepository user.UserRepoInterface) *FriendApplication {
	return &FriendApplication{
		friendRequestRepository: friendRequestRepository,
		userRepository:          userRepository,
	}
}

func (fa *FriendApplication) NewFriendRequest(userId, toUserId, message string) (*friendRequestDTO, error) {
	// TODO 查询 user 数据库，未来可以转换成 gprc 去升级为微服务
	user, err := fa.userRepository.FindUserByUserId(toUserId)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("用户不存在")
	}

	record, err := fa.friendRequestRepository.FindByUsers(userId, toUserId)
	if err != nil {
		return nil, err
	}

	if record == nil {
		// 建立新的申请对象
		newFriendRequest := friend_request_entity.NewFriendRequest(userId, toUserId, message)

		// 入库
		newRecord, err := fa.friendRequestRepository.Create(newFriendRequest)
		if err != nil {
			return nil, err
		}

		return &friendRequestDTO{
			FromUserId: newRecord.FromUserId,
			ToUserId:   newRecord.ToUserId,
			Message:    newRecord.Message,
			Status:     int(newRecord.Status),
			ApplyTime:  newRecord.ApplyTime,
		}, nil
	} else {
		// 有记录
		if err := record.ReApply(message); err != nil {
			return nil, err
		}

		return &friendRequestDTO{
			FromUserId: record.FromUserId,
			ToUserId:   record.ToUserId,
			Message:    record.Message,
			Status:     int(record.Status),
			ApplyTime:  record.ApplyTime,
		}, nil
	}
}
