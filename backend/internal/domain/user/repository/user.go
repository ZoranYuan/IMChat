package user_repository_interface

import (
	user_entity "IM_backend/internal/domain/user/entity"
	"IM_backend/internal/infrastructure/database/mysql/model"

	"gorm.io/gorm"
)

// TODO 定义 user_repository 的接口
type UserRepoInterface interface {
	FindUserByPhone(string) (*model.User, error)
	Create(*model.User) error
	FindByUserId(userId string) (*model.User, error)
	FindByUserIds(userIds []string) ([]user_entity.User, error)
	UpdateByUserIdAndPhone(string, string, map[string]interface{}) error
	WithTx(tx *gorm.DB) UserRepoInterface
}
