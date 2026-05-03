package user

import (
	"IM_backend/configs"
	authport "IM_backend/internal/application/ports/persistence/cache/auth"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	authservice "IM_backend/internal/application/ports/service"
	userentity "IM_backend/internal/domain/user/entity"
	uservo "IM_backend/internal/domain/user/value_object"
	"IM_backend/internal/infrastructure/id/snow"
	usermysql "IM_backend/internal/infrastructure/persistence/mysql/repository/user"
	"context"
	"errors"
	"log"
	"time"

	"gorm.io/gorm"
)

type UserApplication struct {
	userRepository userrepo.UserRepository
	authService    authservice.AuthService
	config         configs.Config
	authCache      authport.AuthCache
}

func NewUserApplication(userRepository userrepo.UserRepository, config configs.Config, authCache authport.AuthCache, authService authservice.AuthService) *UserApplication {
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

	if user != nil {
		return nil, ErrUserAlreadyExists
	}

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("failed to register user, ", err)
			return nil, err
		}
	}

	if password != reconfirmPassword {
		return nil, ErrPasswordMismatch
	}

	newUser, err := userentity.RegisterWithPhone(
		uservo.Phone(phone),
		uservo.Password(password),
	)

	if err != nil {
		return nil, err
	}

	// 生成 UserId
	userId, err := snow.GenerateSnowID(int(ua.config.App.MachineID))
	if err != nil {
		return nil, err
	}

	newUser.UserId = userId
	err = ua.userRepository.Create(usermysql.ToUserModel(newUser))

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

	ua.authCache.SetAccessToken(ctx, accessToken, userId, time.Duration(ua.config.JWT.AccessExpireMinutes)*time.Minute)
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
		return nil, ErrUserNotFound
	}

	// TODO 删除对应的 token 缓存，这里为了防止刷机，可以加一个用户锁
	if err = userentity.LoginWithPhone(
		uservo.Phone(phone),
		uservo.Password(password),
		userModel.Password,
	); err != nil {
		return nil, err
	}

	userModel.OnLineTime = time.Now()

	updates := map[string]interface{}{
		"on_line_time": userModel.OnLineTime,
	}

	if err = ua.userRepository.UpdateByUserIDAndPhone(phone, userModel.UserId, updates); err != nil {
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

func (ua *UserApplication) GetUserByID(userId string) (*UserAppDTO, error) {
	userModel, err := ua.userRepository.FindByUserID(userId)

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
	userModel, err := ua.userRepository.FindByUserID(userId)
	if err != nil {
		return err
	}

	if userModel == nil {
		return ErrUserNotFound
	}

	now := time.Now()

	updates := map[string]interface{}{
		"off_line_time": now,
	}
	return ua.userRepository.UpdateByUserIDAndPhone(userModel.Phone, userModel.UserId, updates)
}
