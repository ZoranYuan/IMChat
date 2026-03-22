package repository

import "gorm.io/gorm"

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *userRepo {
	return &userRepo{
		db: db,
	}
}
