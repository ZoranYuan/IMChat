package user_repository

import (
	user_entity "IM_backend/internal/domain/user/entity"
	user_valueobject "IM_backend/internal/domain/user/value_object"
	"IM_backend/internal/infrastructure/database/mysql/model"
	"errors"

	"gorm.io/gorm"
)

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *userRepo {
	return &userRepo{
		db: db,
	}
}

func (ur *userRepo) FindUserByPhone(phone string) (*model.User, error) {
	var user = model.User{}
	if err := ur.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (ur *userRepo) Create(user *model.User) error {
	result := ur.db.Where("phone = ?", user.Phone).FirstOrCreate(user)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("该账户已被注册")
	}

	return nil
}

func (ur *userRepo) FindUserByUserId(userId string) (*model.User, error) {
	var user = model.User{}
	if err := ur.db.Where("userId = ?", userId).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (ur *userRepo) UpdateByUserIdAndPhone(phone string, userId string, updates map[string]interface{}) error {
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
