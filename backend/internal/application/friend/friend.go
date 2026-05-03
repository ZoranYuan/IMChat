package friend

import (
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	friendvo "IM_backend/internal/domain/friend/value_object"
)

type FriendApplication struct {
	friendRepository friendrepo.FriendRepository
	userRepository   userrepo.UserRepository
}

func NewFriendApplication(
	friendRepository friendrepo.FriendRepository,
	userRepository userrepo.UserRepository,
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
		int(friendvo.DeleteOther),
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
