package persistence

import (
	"context"

	"gorm.io/gorm"
)

type gormTxManager struct {
	db *gorm.DB
}

func NewGormTxManager(db *gorm.DB) *gormTxManager {
	return &gormTxManager{
		db: db,
	}
}

func (tm *gormTxManager) WithinTransaction(
	ctx context.Context,
	fn func(tx any) error,
) error {
	return tm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}
