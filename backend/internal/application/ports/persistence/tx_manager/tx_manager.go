package txmanager

import "context"

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(tx any) error) error
}
