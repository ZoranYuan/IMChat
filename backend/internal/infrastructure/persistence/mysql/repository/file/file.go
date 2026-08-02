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

func (r *FileRepository) WithTx(tx any) filerepo.FileRepository {
	return &FileRepository{db: tx.(*gorm.DB)}
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

func (r *FileRepository) FindByUploaderAndHash(ctx context.Context, uploaderId string, fileHash string) (*fileentity.File, error) {
	if uploaderId == "" || fileHash == "" {
		return nil, nil
	}
	var m model.File
	if err := r.db.WithContext(ctx).
		Where("uploader_id = ? AND file_hash = ?", uploaderId, fileHash).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toFileDomain(&m), nil
}

func (r *FileRepository) BatchGetByIDs(ctx context.Context, fileIds []string) (map[string]*fileentity.File, error) {
	if len(fileIds) == 0 {
		return map[string]*fileentity.File{}, nil
	}
	var models []model.File
	if err := r.db.WithContext(ctx).Where("file_id IN (?)", fileIds).Find(&models).Error; err != nil {
		return nil, err
	}
	result := make(map[string]*fileentity.File, len(models))
	for i := range models {
		result[models[i].FileId] = toFileDomain(&models[i])
	}
	return result, nil
}
