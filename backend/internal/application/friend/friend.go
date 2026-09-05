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
		int(friendvo.Friend),
	)
	if err != nil {
		return nil, err
	}

	userIds := make([]string, 0, len(friends))

	for _, f := range friends {
		userIds = append(userIds, f.FriendUserId)
	}

	users, err := fa.userRepository.FindByUserIDs(userIds)

	if err != nil {
		return nil, err
	}

	userByID := make(map[string]struct {
		avatar   string
		userName string
		nickName string
	}, len(users))
	for _, user := range users {
		userByID[user.UserId] = struct {
			avatar   string
			userName string
			nickName string
		}{
			avatar:   user.Avatar,
			userName: user.UserName,
			nickName: user.NickName,
		}
	}

	friendListApp := make([]FriendAppDTO, 0, len(friends))
	for _, friend := range friends {
		user, ok := userByID[friend.FriendUserId]
		if !ok {
			continue
		}
		friendListApp = append(friendListApp, FriendAppDTO{
			FriendUserID:    friend.FriendUserId,
			FriendAvatarURL: user.avatar,
			FriendUsername:  user.userName,
			FriendNickname:  user.nickName,
			Status:          int(friend.Status),
			Remarks:         friend.Remarks,
		})
	}

	return friendListApp, nil
}
