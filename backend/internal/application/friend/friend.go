package application_friend

import (
	friend_repository_interface "IM_backend/internal/application/ports/repository/friend"
	user_repository_interface "IM_backend/internal/application/ports/repository/user"
	friend_valueobject "IM_backend/internal/domain/friend/value_object"
)

type FriendApplication struct {
	friendRepository friend_repository_interface.FriendRepository
	userRepository   user_repository_interface.UserRepository
}

func NewFriendApplication(
	friendRepository friend_repository_interface.FriendRepository,
	userRepository user_repository_interface.UserRepository,
) *FriendApplication {
	return &FriendApplication{
		friendRepository: friendRepository,
		userRepository:   userRepository,
	}
}

func (fa *FriendApplication) GetUserFriendList(userId string) ([]FriendAppDTO, error) {
	// TODO 权衡这里是否有必要加入 userId 的查询，判断当前用户是否存在
	friends, err := fa.friendRepository.GetUserFriendList(
		userId,
		int(friend_valueobject.DeleteOther),
	)

	userIds := make([]string, 0, len(friends))

	for _, f := range friends {
		userIds = append(userIds, f.FriendUserId)
	}

	users, err := fa.userRepository.FindByUserIDs(userIds)

	if err != nil {
		return nil, err
	}

	friendListApp := make([]FriendAppDTO, 0, len(users))
	for i := range users {
		friendListApp = append(friendListApp, FriendAppDTO{
			FriendUserId:   friends[i].FriendUserId,
			FriendAvatar:   users[i].Avatar,
			FriendUserName: users[i].UserName,
			FriendNickName: users[i].NickName,
			Status:         int(friends[i].Status),
			Remarks:        friends[i].Remarks,
		})
	}

	return friendListApp, nil
}
