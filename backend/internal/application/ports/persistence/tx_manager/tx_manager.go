package tx_repository_interface

import (
	"context"

	"gorm.io/gorm"
)

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
}
