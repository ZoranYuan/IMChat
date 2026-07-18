package user

import (
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	userentity "IM_backend/internal/domain/user/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"errors"
	"time"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) WithTx(tx any) userrepo.UserRepository {
	return &UserRepository{db: tx.(*gorm.DB)}
}

func (ur *UserRepository) FindUserByPhone(phone string) (*userentity.User, error) {
	var user = model.User{}
	if err := ur.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	domain := toDomain(user)
	return &domain, nil
}

func (ur *UserRepository) Create(user *userentity.User) error {
	model := toModel(user)
	result := ur.db.Where("phone = ?", model.Phone).FirstOrCreate(model)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return userentity.ErrUserAlreadyExists
	}

	return nil
}

func (ur *UserRepository) FindByUserID(userId string) (*userentity.User, error) {
	var user = model.User{}
	if err := ur.db.Where("user_id = ?", userId).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	domain := toDomain(user)
	return &domain, nil
}

func (ur *UserRepository) FindByUsernameOrPhone(keyword string) (*userentity.User, error) {
	var user = model.User{}
	if err := ur.db.Where("user_name = ? OR phone = ?", keyword, keyword).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	domain := toDomain(user)
	return &domain, nil
}

func (ur *UserRepository) FindByUserIDs(userIds []string) ([]userentity.User, error) {
	if len(userIds) == 0 {
		return []userentity.User{}, nil
	}

	var models []model.User

	err := ur.db.
		Where("user_id IN ?", userIds).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	users := make([]userentity.User, 0, len(models))
	for _, m := range models {
		users = append(users, toDomain(m))
	}

	return users, nil
}

func (ur *UserRepository) updateTime(userId, phone, column string, value time.Time) error {
	return ur.db.Model(&model.User{}).
		Where("user_id = ? AND phone = ?", userId, phone).
		Update(column, value).
		Error
}

func (ur *UserRepository) UpdateOnlineTime(userId, phone string, onlineAt time.Time) error {
	return ur.updateTime(userId, phone, "on_line_time", onlineAt)
}

func (ur *UserRepository) UpdateOfflineTime(userId, phone string, offlineAt time.Time) error {
	return ur.updateTime(userId, phone, "off_line_time", offlineAt)
}

func toModel(u *userentity.User) *model.User {
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
		LoginType:   int(u.LoginType),
		WxOpenID:    u.WxOpenID,
		WxUnionID:   u.WxUnionID,
	}
}
