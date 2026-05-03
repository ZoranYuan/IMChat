package user_entity

import "errors"

var (
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	errIncorrectPassword  = errors.New("incorrect password")
	errInvalidPhoneNumber = errors.New("invalid phone number")
)
