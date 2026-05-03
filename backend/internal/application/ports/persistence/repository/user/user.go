package user

import (
	userentity "IM_backend/internal/domain/user/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

	"gorm.io/gorm"
)

// TODO 定义 usermysql 的接口
type UserRepository interface {
	FindUserByPhone(string) (*model.User, error)
	Create(*model.User) error
	FindByUserID(userId string) (*model.User, error)
	FindByUserIDs(userIds []string) ([]userentity.User, error)
	UpdateByUserIDAndPhone(string, string, map[string]interface{}) error
	WithTx(tx *gorm.DB) UserRepository
}
