package user

import (
	"IM_backend/internal/application/ports/persistence/cache/user"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"encoding/json"
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
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}
	if userProfile.UserID == "" {
		userProfile.UserID = userId
	}

	return &userProfile, true, nil
}

func (u *UserCache) GetUserProfiles(
	ctx context.Context,
	userIds []string,
) map[string]*user.UserProfile {
	userProfileM := make(map[string]*user.UserProfile, len(userIds))
	if len(userIds) == 0 {
		return userProfileM
	}

	keys := make([]string, 0, len(userIds))
	keyUserIds := make([]string, 0, len(userIds))
	seen := make(map[string]struct{}, len(userIds))
	for _, userId := range userIds {
		if userId == "" {
			continue
		}
		if _, ok := seen[userId]; ok {
			continue
		}
		seen[userId] = struct{}{}
		keys = append(keys, UserProfileKey(userId))
		keyUserIds = append(keyUserIds, userId)
	}
	if len(keys) == 0 {
		return userProfileM
	}

	// 使用 MGET ，获取一批 key
	values, err := u.store.Client().MGet(ctx, keys...).Result()
	if err != nil {
		return userProfileM
	}

	for i, value := range values {
		if value == nil {
			continue
		}
		var raw []byte
		switch v := value.(type) {
		case string:
			raw = []byte(v)
		case []byte:
			raw = v
		default:
			continue
		}
		var userProfile user.UserProfile
		if err := json.Unmarshal(raw, &userProfile); err != nil {
			continue
		}
		userId := keyUserIds[i]
		if userProfile.UserID == "" {
			userProfile.UserID = userId
		}
		userProfileM[userId] = &userProfile
	}

	return userProfileM
}

func (u *UserCache) SetUserProfile(
	ctx context.Context,
	profile *user.UserProfile,
	ttl time.Duration,
) error {
	if profile == nil || profile.UserID == "" {
		return ErrEmptyUserId
	}
	profile.Found = true
	return u.store.SetJSON(ctx, UserProfileKey(profile.UserID), profile, ttl)
}

func (u *UserCache) SetUserProfiles(
	ctx context.Context,
	profiles []*user.UserProfile,
	ttl time.Duration,
) error {
	if len(profiles) == 0 {
		return nil
	}

	pipe := u.store.Client().Pipeline()
	for _, profile := range profiles {
		if profile == nil || profile.UserID == "" {
			continue
		}

		profile.Found = true
		data, err := json.Marshal(profile)
		if err != nil {
			return err
		}
		pipe.Set(ctx, UserProfileKey(profile.UserID), data, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (u *UserCache) SetUserProfileNotFound(
	ctx context.Context,
	userId string,
	ttl time.Duration,
) error {
	if userId == "" {
		return ErrEmptyUserId
	}
	return u.store.SetJSON(ctx, UserProfileKey(userId), &user.UserProfile{Found: false, UserID: userId}, ttl)
}

func (u *UserCache) SetUserProfilesNotFound(
	ctx context.Context,
	userIds []string,
	ttl time.Duration,
) error {
	if len(userIds) == 0 {
		return nil
	}

	pipe := u.store.Client().Pipeline()
	seen := make(map[string]struct{}, len(userIds))
	for _, userId := range userIds {
		if userId == "" {
			continue
		}
		if _, ok := seen[userId]; ok {
			continue
		}
		seen[userId] = struct{}{}

		data, err := json.Marshal(&user.UserProfile{Found: false, UserID: userId})
		if err != nil {
			return err
		}
		pipe.Set(ctx, UserProfileKey(userId), data, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
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
