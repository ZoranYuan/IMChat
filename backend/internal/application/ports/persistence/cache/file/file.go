package file

import (
	fileentity "IM_backend/internal/domain/file/entity"
	"context"
	"time"
)

type FileCache interface {
	Set(ctx context.Context, file *fileentity.File, ttl time.Duration) error
	Get(ctx context.Context, fileId string) (*fileentity.File, error)
}
