package user

import (
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	userentity "IM_backend/internal/domain/user/entity"
	uservo "IM_backend/internal/domain/user/value_object"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

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

func (r *UserRepository) WithTx(tx *gorm.DB) userrepo.UserRepository {
	return &UserRepository{db: tx}
}

func (ur *UserRepository) FindUserByPhone(phone string) (*model.User, error) {
	var user = model.User{}
	if err := ur.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (ur *UserRepository) Create(user *model.User) error {
	result := ur.db.Where("phone = ?", user.Phone).FirstOrCreate(user)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return userentity.ErrUserAlreadyExists
	}

	return nil
}

func (ur *UserRepository) FindByUserID(userId string) (*model.User, error) {
	var user = model.User{}
	if err := ur.db.Where("user_id = ?", userId).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
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

func (ur *UserRepository) UpdateByUserIDAndPhone(phone string, userId string, updates map[string]interface{}) error {
	return ur.db.Model(&model.User{}).
		Where("user_id = ? AND phone = ?", userId, phone).
		Updates(updates).
		Error
}

func ToUserModel(u *userentity.User) *model.User {
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

func ToUserEntity(m *model.User) *userentity.User {
	return &userentity.User{
		UserId:      m.UserId,
		UserName:    m.UserName,
		NickName:    m.NickName,
		Avatar:      m.Avatar,
		Password:    uservo.Password(m.Password),
		Phone:       uservo.Phone(m.Phone),
		Status:      uservo.Status(m.Status),
		OnLineTime:  m.OnLineTime,
		OffLineTime: m.OffLineTime,
	}
}
