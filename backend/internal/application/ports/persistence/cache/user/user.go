package user

import (
	"context"
	"time"
)

type UserProfile struct {
	UserID   string
	UserName string
	NickName string
	Avatar   string
	Status   int
}

type UserCache interface {
	GetUserProfile(
		ctx context.Context,
		userId string,
	) (*UserProfile, bool, error)

	GetUserProfiles(
		ctx context.Context,
		userIds []string,
	) map[string]*UserProfile

	SetUserProfile(
		ctx context.Context,
		profile *UserProfile,
		ttl time.Duration,
	) error

	DeleteUserProfiles(
		ctx context.Context,
		userIds []string,
	) error
}
