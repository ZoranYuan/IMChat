package application_user

import (
	"IM_backend/configs"
	"IM_backend/internal/domain/user"
	user_entity "IM_backend/internal/domain/user/entity"
	user_valueobject "IM_backend/internal/domain/user/value_object"
	user_repository "IM_backend/internal/infrastructure/database/mysql/repository"
	"IM_backend/internal/infrastructure/pkg/jwt"
	"IM_backend/internal/infrastructure/pkg/snow"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

type UserApplication struct {
	userRepository user.UserRepoInterface
	config         configs.Config
}

func NewUserApplication(userRepository user.UserRepoInterface, config configs.Config) *UserApplication {
	return &UserApplication{
		userRepository: userRepository,
		config:         config,
	}
}

func (ua *UserApplication) RegisterWithPhone(password string, phone string, reconfirmPassword string) (*UserAppDTO, string, error) {
	// TODO 检查当前用户是否存在
	user, err := ua.userRepository.FindUserByPhone(phone)

	fmt.Println("user is ", user)
	if user != nil {
		return nil, "", errors.New("账号已被注册，请返回登录")
	}

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("failed to register user, ", err)
			return nil, "", err
		}
	}

	if password != reconfirmPassword {
		return nil, "", errors.New("两次密码不一致")
	}

	newUser, err := user_entity.RegisterWithPhone(
		user_valueobject.Phone(phone),
		user_valueobject.Password(password),
	)

	if err != nil {
		return nil, "", err
	}

	// 生成 UserId
	userId, err := snow.GenerateSnowId(int(ua.config.Snowflake.MachineID))
	if err != nil {
		return nil, "", err
	}

	newUser.UserId = userId
	err = ua.userRepository.Create(user_repository.ToUserModel(newUser))

	if err != nil {
		return nil, "", err
	}

	// 生成 token
	token, err := jwt.GenerateToken(userId, ua.config.JWT.Secret, time.Duration(ua.config.JWT.AccessExpireHours))
	if err != nil {
		return nil, "", err
	}

	userApp := &UserAppDTO{
		UserId:   newUser.UserId,
		UserName: newUser.UserName,
		NickName: newUser.NickName,
		Avatar:   newUser.Avatar,
		Phone:    string(newUser.Phone),
	}

	return userApp, token, nil
}

func (ua *UserApplication) LoginWithPhone(phone, password string) (*UserAppDTO, string, error) {
	userModel, err := ua.userRepository.FindUserByPhone(phone)

	if err != nil {
		log.Println("failed to register user, ", err)
		return nil, "", err
	}

	if userModel == nil {
		return nil, "", errors.New("该账户为注册")
	}

	// TODO 删除对应的 token 缓存，这里为了防止刷机，可以加一个用户锁

	if err = user_entity.LoginWithPhone(
		user_valueobject.Phone(phone),
		user_valueobject.Password(password),
		userModel.Password,
	); err != nil {
		return nil, "", err
	}

	userModel.OnLineTime = time.Now()

	updates := map[string]interface{}{
		"on_line_time": userModel.OnLineTime,
	}

	if err = ua.userRepository.Update(phone, userModel.UserId, updates); err != nil {
		return nil, "", err
	}

	// 生成 token
	token, err := jwt.GenerateToken(userModel.UserId, ua.config.JWT.Secret, time.Duration(ua.config.JWT.AccessExpireHours))
	if err != nil {
		return nil, "", err
	}

	userAppDTO := &UserAppDTO{
		UserId:   userModel.UserId,
		UserName: userModel.UserName,
		NickName: userModel.NickName,
		Avatar:   userModel.Avatar,
		Phone:    userModel.Phone,
	}

	// 更新缓存
	return userAppDTO, token, nil
}
