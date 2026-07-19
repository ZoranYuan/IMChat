package user

import (
	"IM_backend/internal/application/ports/persistence/cache/user"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type UserCache struct {
	store *shared.Store
}

func NewUserCache(client *redis.Client) user.UserCache {
	return &UserCache{store: shared.NewStore(client)}
}

func (u *UserCache) GetUserProfile(
	ctx context.Context,
	userId string,
) (*user.UserProfile, bool, error) {
	if userId == "" {
		return nil, false, ErrEmptyUserId
	}

	userProfile := user.UserProfile{}
	exist, err := u.store.GetJSON(ctx, UserProfileKey(userId), &userProfile)

	if !exist && err == nil {
		// 用户不存在
		return nil, false, ErrUserNotFonud
	}

	if err != nil {
		return nil, false, err
	}

	return &userProfile, true, nil
}

func (u *UserCache) GetUserProfiles(
	ctx context.Context,
	userIds []string,
) map[string]*user.UserProfile {
	userProfileM := make(map[string]*user.UserProfile, len(userIds))

	for _, userId := range userIds {
		userProfile, _, _ := u.GetUserProfile(ctx, userId)
		userProfileM[userId] = userProfile
	}

	return userProfileM
}

func (u *UserCache) SetUserProfile(
	ctx context.Context,
	profile *user.UserProfile,
	ttl time.Duration,
) error {
	return u.store.SetJSON(ctx, UserProfileKey(profile.UserID), profile, ttl)
}

func (u *UserCache) DeleteUserProfiles(
	ctx context.Context,
	userIds []string,
) error {
	keysM := make(map[string]struct{})
	for _, userId := range userIds {
		keysM[UserProfileKey(userId)] = struct{}{}
	}

	keysSet := make([]string, 0)

	for key := range keysM {
		keysSet = append(keysSet, key)
	}

	return u.store.Del(ctx, keysSet...)
}
