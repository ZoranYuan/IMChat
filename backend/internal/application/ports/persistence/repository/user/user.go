package user

import (
	userentity "IM_backend/internal/domain/user/entity"
	"context"
	"time"
)

type UserRepository interface {
	FindUserByPhone(string) (*userentity.User, error)
	Create(*userentity.User) error
	FindByUserID(userId string) (*userentity.User, error)
	FindByUsernameOrPhone(keyword string) (*userentity.User, error)
	FindByUserIDs(userIds []string) ([]userentity.User, error)
	UpdateUserProfile(ctx context.Context, userId string, updates map[string]any) (*userentity.User, error)
	UpdateOnlineTime(userId, phone string, onlineAt time.Time) error
	UpdateOfflineTime(userId, phone string, offlineAt time.Time) error
	WithTx(tx any) UserRepository
}
