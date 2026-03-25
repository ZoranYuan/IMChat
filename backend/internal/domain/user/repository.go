package user

import "IM_backend/internal/infrastructure/database/mysql/model"

// TODO 定义 user_repository 的接口
type UserRepoInterface interface {
	FindUserByPhone(string) (*model.User, error)
	Create(*model.User) error
	FindUserByUserId(userId string) (*model.User, error)
	UpdateByUserIdAndPhone(string, string, map[string]interface{}) error
}
