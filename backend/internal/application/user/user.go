package user

import (
	idport "IM_backend/internal/application/ports/id"
	authport "IM_backend/internal/application/ports/persistence/cache/auth"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	securityport "IM_backend/internal/application/ports/security"
	userentity "IM_backend/internal/domain/user/entity"
	uservo "IM_backend/internal/domain/user/value_object"
	"context"
	"log"
	"time"
)

type UserApplication struct {
	userRepository userrepo.UserRepository
	tokenIssuer    securityport.TokenIssuer
	options        Options
	authCache      authport.AuthCache
	idGenerator    idport.Generator
	passwordHasher securityport.PasswordHasher
}

type Options struct {
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func NewUserApplication(userRepository userrepo.UserRepository, options Options, authCache authport.AuthCache, tokenIssuer securityport.TokenIssuer, idGenerator idport.Generator, passwordHasher securityport.PasswordHasher) *UserApplication {
	return &UserApplication{
		userRepository: userRepository,
		options:        options,
		authCache:      authCache,
		tokenIssuer:    tokenIssuer,
		idGenerator:    idGenerator,
		passwordHasher: passwordHasher,
	}
}

func (ua *UserApplication) issueTokensAndCache(userId string) (string, string, error) {
	accessToken, refreshToken, sessionID, err := ua.tokenIssuer.IssueToken(userId)
	if err != nil {
		return "", "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := ua.authCache.SetRefreshSession(ctx, refreshToken, authport.RefreshSession{
		UserID:    userId,
		SessionID: sessionID,
	}, ua.options.RefreshTokenTTL); err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (ua *UserApplication) RegisterWithPhone(password string, phone string, reconfirmPassword string) (*UserAppDTO, error) {
	if !uservo.Phone(phone).Validate() {
		return nil, ErrInvalidPhoneNumber
	}

	user, err := ua.userRepository.FindUserByPhone(phone)
	if err != nil {
		log.Println("注册用户失败：", err)
		return nil, err
	}
	if user != nil {
		return nil, ErrUserAlreadyExists
	}

	if password != reconfirmPassword {
		return nil, ErrPasswordMismatch
	}

	hashedPassword, err := ua.passwordHasher.Hash(password)
	if err != nil {
		return nil, err
	}

	newUser, err := userentity.RegisterWithPhone(
		uservo.Phone(phone),
		uservo.Password(hashedPassword),
	)

	if err != nil {
		return nil, err
	}

	// 生成 UserId
	userId, err := ua.idGenerator.Generate()
	if err != nil {
		return nil, err
	}

	newUser.UserId = userId
	err = ua.userRepository.Create(newUser)

	if err != nil {
		return nil, err
	}

	// 生成 token
	accessToken, refreshToken, err := ua.issueTokensAndCache(userId)
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

	return userApp, nil
}

func (ua *UserApplication) LoginWithPhone(phone, password string) (*UserAppDTO, error) {
	if !uservo.Phone(phone).Validate() {
		return nil, ErrInvalidPhoneNumber
	}

	user, err := ua.userRepository.FindUserByPhone(phone)

	if err != nil {
		log.Println("手机号登录失败：", err)
		return nil, err
	}

	if user == nil {
		return nil, ErrUserNotFound
	}

	if !ua.passwordHasher.Verify(password, string(user.Password)) {
		return nil, ErrIncorrectPassword
	}

	user.OnLineTime = time.Now()
	if err = ua.userRepository.UpdateOnlineTime(user.UserId, string(user.Phone), user.OnLineTime); err != nil {
		return nil, err
	}

	// 生成 token
	accessToken, refreshToken, err := ua.issueTokensAndCache(user.UserId)
	if err != nil {
		return nil, err
	}

	userAppDTO := &UserAppDTO{
		UserId:       user.UserId,
		UserName:     user.UserName,
		NickName:     user.NickName,
		Avatar:       user.Avatar,
		Phone:        string(user.Phone),
		RefreshToken: refreshToken,
		AccessToken:  accessToken,
	}

	return userAppDTO, nil
}

func (ua *UserApplication) LoginWithUserName(keyword, password string) (*UserAppDTO, error) {
	user, err := ua.userRepository.FindByUsernameOrPhone(keyword)

	if err != nil {
		log.Println("用户名登录失败：", err)
		return nil, err
	}

	if user == nil {
		return nil, ErrUserNotFound
	}

	if !ua.passwordHasher.Verify(password, string(user.Password)) {
		return nil, ErrIncorrectPassword
	}

	user.OnLineTime = time.Now()
	if err = ua.userRepository.UpdateOnlineTime(user.UserId, string(user.Phone), user.OnLineTime); err != nil {
		return nil, err
	}

	accessToken, refreshToken, err := ua.issueTokensAndCache(user.UserId)
	if err != nil {
		return nil, err
	}

	userAppDTO := &UserAppDTO{
		UserId:       user.UserId,
		UserName:     user.UserName,
		NickName:     user.NickName,
		Avatar:       user.Avatar,
		Phone:        string(user.Phone),
		RefreshToken: refreshToken,
		AccessToken:  accessToken,
	}

	return userAppDTO, nil
}

func (ua *UserApplication) GetUserByID(userId string) (*UserAppDTO, error) {
	user, err := ua.userRepository.FindByUserID(userId)

	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	var userApp = &UserAppDTO{
		UserId:   user.UserId,
		UserName: user.UserName,
		NickName: user.NickName,
		Phone:    string(user.Phone),
		Avatar:   user.Avatar,
	}

	return userApp, nil
}

func (ua *UserApplication) ResolveUser(keyword string) (*UserAppDTO, error) {
	user, err := ua.userRepository.FindByUsernameOrPhone(keyword)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return &UserAppDTO{
		UserId:   user.UserId,
		UserName: user.UserName,
		NickName: user.NickName,
		Phone:    string(user.Phone),
		Avatar:   user.Avatar,
	}, nil
}

func (ua *UserApplication) Logout(userId, refreshToken string) error {
	user, err := ua.userRepository.FindByUserID(userId)
	if err != nil {
		return err
	}

	if user == nil {
		return ErrUserNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if refreshToken != "" {
		if err := ua.authCache.DeleteRefreshSession(ctx, refreshToken); err != nil {
			return err
		}
	}
	return ua.userRepository.UpdateOfflineTime(user.UserId, string(user.Phone), time.Now())
}

func (ua *UserApplication) Refresh(refreshToken string) (*UserAppDTO, error) {
	if refreshToken == "" {
		return nil, ErrUserNotFound
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	session, found, err := ua.authCache.GetRefreshSession(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	if !found || session.UserID == "" {
		return nil, ErrUserNotFound
	}
	user, err := ua.userRepository.FindByUserID(session.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	access, refresh, sessionID, err := ua.tokenIssuer.IssueToken(session.UserID)
	if err != nil {
		return nil, err
	}
	rotated, err := ua.authCache.RotateRefreshSession(ctx, refreshToken, refresh, authport.RefreshSession{
		UserID:    session.UserID,
		SessionID: sessionID,
	}, ua.options.RefreshTokenTTL)
	if err != nil {
		return nil, err
	}
	if !rotated {
		return nil, ErrUserNotFound
	}
	return &UserAppDTO{UserId: user.UserId, UserName: user.UserName, NickName: user.NickName, Phone: string(user.Phone), Avatar: user.Avatar, AccessToken: access, RefreshToken: refresh}, nil
}
