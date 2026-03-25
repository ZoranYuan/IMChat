package user_entity

import "errors"

var (
	errUserNotFound      = errors.New("user not found")
	errWrongPassword     = errors.New("wrong password")
	errNickNameTooLong   = errors.New("nickName too long")
	errUserAlreadyExists = errors.New("user already exists")
	errInvalidEmail      = errors.New("invalid email")
	errInvalidPhone      = errors.New("invalid phone")
)
