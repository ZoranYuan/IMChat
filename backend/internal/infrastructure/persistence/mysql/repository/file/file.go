package file

import (
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	fileentity "IM_backend/internal/domain/file/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"context"
	"errors"

	"gorm.io/gorm"
)

type FileRepository struct {
	db *gorm.DB
}

func NewFileRepository(db *gorm.DB) filerepo.FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Save(ctx context.Context, file *fileentity.File) error {
	return r.db.WithContext(ctx).Create(toFileModel(file)).Error
}

func (r *FileRepository) GetByID(ctx context.Context, fileId string) (*fileentity.File, error) {
	var m model.File
	if err := r.db.WithContext(ctx).Where("file_id = ?", fileId).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toFileDomain(&m), nil
}
