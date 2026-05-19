package file

import (
	fileentity "IM_backend/internal/domain/file/entity"
	"context"
)

type FileRepository interface {
	Save(ctx context.Context, file *fileentity.File) error
	GetByID(ctx context.Context, fileId string) (*fileentity.File, error)
}
