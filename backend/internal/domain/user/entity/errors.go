package user_entity

import "errors"

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrWrongPassword     = errors.New("wrong password")
	ErrNickNameTooLong   = errors.New("nickName too long")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidEmail      = errors.New("invalid email")
	ErrInvalidPhone      = errors.New("invalid phone")
)
