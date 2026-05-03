package application_user

import "errors"

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrPasswordMismatch  = errors.New("passwords do not match")
	ErrUserNotFound      = errors.New("user not found")
)
