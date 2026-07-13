package entity

import "errors"

var (
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrIncorrectPassword  = errors.New("incorrect password")
	ErrInvalidPhoneNumber = errors.New("invalid phone number")
)
