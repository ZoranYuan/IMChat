package application_user

import (
	"IM_backend/configs"
	auth_cache_interface "IM_backend/internal/applications/interface/cache/auth"
	auth_service_interface "IM_backend/internal/applications/interface/service"
	user_entity "IM_backend/internal/domain/user/entity"
	user_repository_interface "IM_backend/internal/domain/user/repository"
	user_valueobject "IM_backend/internal/domain/user/value_object"
	user_repository "IM_backend/internal/infrastructure/database/mysql/repository/user"
	"IM_backend/internal/infrastructure/pkg/snow"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

type UserApplication struct {
	userRepository user_repository_interface.UserRepoInterface
	authService    auth_service_interface.AuthService
	config         configs.Config
	authCache      auth_cache_interface.AuthCacheInterface
}

func NewUserApplication(userRepository user_repository_interface.UserRepoInterface, config configs.Config, authCache auth_cache_interface.AuthCacheInterface, authService auth_service_interface.AuthService) *UserApplication {
	return &UserApplication{
		userRepository: userRepository,
		config:         config,
		authCache:      authCache,
		authService:    authService,
	}
}

func (ua *UserApplication) RegisterWithPhone(password string, phone string, reconfirmPassword string) (*UserAppDTO, error) {
	// TODO 检查当前用户是否存在
	user, err := ua.userRepository.FindUserByPhone(phone)

	fmt.Println("user is ", user)
	if user != nil {
		return nil, errors.New("账号已被注册，请返回登录")
	}

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("failed to register user, ", err)
			return nil, err
		}
	}

	if password != reconfirmPassword {
		return nil, errors.New("两次密码不一致")
	}

	newUser, err := user_entity.RegisterWithPhone(
		user_valueobject.Phone(phone),
		user_valueobject.Password(password),
	)

	if err != nil {
		return nil, err
	}

	// 生成 UserId
	userId, err := snow.GenerateSnowId(int(ua.config.App.MachineID))
	if err != nil {
		return nil, err
	}

	newUser.UserId = userId
	err = ua.userRepository.Create(user_repository.ToUserModel(newUser))

	if err != nil {
		return nil, err
	}

	// 生成 token
	accessToken, refreshToken, err := ua.authService.IssueToken(userId)

	if err != nil {
		return nil, err
	}

	userApp := &UserAppDTO{
		UserId:       newUser.UserId,
		UserName:     newUser.UserName,
		NickName:     newUser.NickName,
		Avatar:       newUser.Avatar,
		Phone:        string(newUser.Phone),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ua.authCache.SetAccessToken(ctx, accessToken, userId, time.Duration(ua.config.JWT.AccessExpireMinutes)*time.Hour)
	ua.authCache.SetRefreshToken(ctx, refreshToken, userId, time.Duration(ua.config.JWT.RefreshExpireHours)*time.Hour)
	return userApp, nil
}

func (ua *UserApplication) LoginWithPhone(phone, password string) (*UserAppDTO, error) {
	userModel, err := ua.userRepository.FindUserByPhone(phone)

	if err != nil {
		log.Println("failed to register user, ", err)
		return nil, err
	}

	if userModel == nil {
		return nil, errors.New("账户不存在")
	}

	// TODO 删除对应的 token 缓存，这里为了防止刷机，可以加一个用户锁
	if err = user_entity.LoginWithPhone(
		user_valueobject.Phone(phone),
		user_valueobject.Password(password),
		userModel.Password,
	); err != nil {
		return nil, err
	}

	userModel.OnLineTime = time.Now()

	updates := map[string]interface{}{
		"on_line_time": userModel.OnLineTime,
	}

	if err = ua.userRepository.UpdateByUserIdAndPhone(phone, userModel.UserId, updates); err != nil {
		return nil, err
	}

	// 生成 token
	accessToken, refreshToken, err := ua.authService.IssueToken(userModel.UserId)

	if err != nil {
		return nil, err
	}

	userAppDTO := &UserAppDTO{
		UserId:       userModel.UserId,
		UserName:     userModel.UserName,
		NickName:     userModel.NickName,
		Avatar:       userModel.Avatar,
		Phone:        userModel.Phone,
		RefreshToken: refreshToken,
		AccessToken:  accessToken,
	}

	// TODO 将之前的缓存删除

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ua.authCache.SetAccessToken(ctx, accessToken, userAppDTO.UserId, time.Duration(ua.config.JWT.AccessExpireMinutes)*time.Minute)
	ua.authCache.SetRefreshToken(ctx, refreshToken, userAppDTO.UserId, time.Duration(ua.config.JWT.RefreshExpireHours)*time.Hour)

	// 更新缓存
	return userAppDTO, nil
}

func (ua *UserApplication) GetUserByUserId(userId string) (*UserAppDTO, error) {
	userModel, err := ua.userRepository.FindByUserId(userId)

	if err != nil {
		return nil, err
	}

	var userApp = &UserAppDTO{
		UserId:   userModel.UserId,
		UserName: userModel.UserName,
		NickName: userModel.NickName,
		Phone:    userModel.Phone,
		Avatar:   userModel.Avatar,
	}

	return userApp, nil
}

func (ua *UserApplication) Logout(userId string) error {
	userModel, err := ua.userRepository.FindByUserId(userId)
	if err != nil {
		return nil
	}

	if userModel == nil {
		return errors.New("账户不存在")
	}

	now := time.Now()

	updates := map[string]interface{}{
		"off_line_time": now,
	}
	return ua.userRepository.UpdateByUserIdAndPhone(userModel.Phone, userModel.UserId, updates)
}
