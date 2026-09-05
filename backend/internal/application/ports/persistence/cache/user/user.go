package user

import (
	"context"
	"time"
)

type UserProfile struct {
	Found    bool
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

	SetUserProfiles(
		ctx context.Context,
		profiles []*UserProfile,
		ttl time.Duration,
	) error

	SetUserProfileNotFound(
		ctx context.Context,
		userId string,
		ttl time.Duration,
	) error

	SetUserProfilesNotFound(
		ctx context.Context,
		userIds []string,
		ttl time.Duration,
	) error

	DeleteUserProfiles(
		ctx context.Context,
		userIds []string,
	) error
}
