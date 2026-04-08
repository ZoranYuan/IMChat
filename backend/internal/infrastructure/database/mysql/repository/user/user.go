package user_repository

import (
	user_entity "IM_backend/internal/domain/user/entity"
	user_valueobject "IM_backend/internal/domain/user/value_object"
	"IM_backend/internal/infrastructure/database/mysql/model"
	"errors"

	"gorm.io/gorm"
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *userRepository {
	return &userRepository{
		db: db,
	}
}

func (r *userRepository) WithTx(tx *gorm.DB) *userRepository {
	return &userRepository{db: tx}
}

func (ur *userRepository) FindUserByPhone(phone string) (*model.User, error) {
	var user = model.User{}
	if err := ur.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (ur *userRepository) Create(user *model.User) error {
	result := ur.db.Where("phone = ?", user.Phone).FirstOrCreate(user)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("该账户已被注册")
	}

	return nil
}

func (ur *userRepository) FindByUserId(userId string) (*model.User, error) {
	var user = model.User{}
	if err := ur.db.Where("user_id = ?", userId).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (ur *userRepository) FindByUserIds(userIds []string) ([]user_entity.User, error) {
	if len(userIds) == 0 {
		return []user_entity.User{}, nil
	}

	var models []model.User

	err := ur.db.
		Where("user_id IN ?", userIds).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	users := make([]user_entity.User, 0, len(models))
	for _, m := range models {
		users = append(users, toDomain(m))
	}

	return users, nil
}

func (ur *userRepository) UpdateByUserIdAndPhone(phone string, userId string, updates map[string]interface{}) error {
	return ur.db.Model(&model.User{}).
		Where("user_id = ? AND phone = ?", userId, phone).
		Updates(updates).
		Error
}

func ToUserModel(u *user_entity.User) *model.User {
	return &model.User{
		UserId:      u.UserId,
		UserName:    u.UserName,
		NickName:    u.NickName,
		Password:    string(u.Password), // ValueObject -> string
		Phone:       string(u.Phone),
		Avatar:      u.Avatar,
		Status:      int(u.Status),
		OnLineTime:  u.OnLineTime,
		OffLineTime: u.OffLineTime,
	}
}

func ToUserEntity(m *model.User) *user_entity.User {
	return &user_entity.User{
		UserId:      m.UserId,
		UserName:    m.UserName,
		NickName:    m.NickName,
		Avatar:      m.Avatar,
		Password:    user_valueobject.Password(m.Password),
		Phone:       user_valueobject.Phone(m.Phone),
		Status:      user_valueobject.Status(m.Status),
		OnLineTime:  m.OnLineTime,
		OffLineTime: m.OffLineTime,
	}
}
