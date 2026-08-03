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
		Where("uploader_id = ? AND file_hash = ? AND status = ?", uploaderId, fileHash, fileentity.FileStatusUploaded).
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

func (r *FileRepository) ListOrphanCandidates(ctx context.Context, before int64, limit int) ([]*fileentity.File, error) {
	if limit <= 0 {
		return []*fileentity.File{}, nil
	}
	var models []model.File
	err := r.db.WithContext(ctx).Table("files AS f").
		Where("f.created_at < ? AND f.status IN ?", before, []string{
			fileentity.FileStatusUploaded,
			fileentity.FileStatusDeleting,
		}).
		Where("NOT EXISTS (SELECT 1 FROM message_attachments ma WHERE ma.file_id = f.file_id)").
		Order("f.created_at ASC").Limit(limit).Find(&models).Error
	if err != nil {
		return nil, err
	}
	result := make([]*fileentity.File, 0, len(models))
	for i := range models {
		result = append(result, toFileDomain(&models[i]))
	}
	return result, nil
}

func (r *FileRepository) MarkDeleting(ctx context.Context, fileId string, before int64) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.File{}).
		Where("file_id = ? AND status = ? AND created_at < ?", fileId, fileentity.FileStatusUploaded, before).
		Where("NOT EXISTS (SELECT 1 FROM message_attachments ma WHERE ma.file_id = ?)", fileId).
		Update("status", fileentity.FileStatusDeleting)
	return result.RowsAffected == 1, result.Error
}

func (r *FileRepository) DeleteDeleting(ctx context.Context, fileId string) (bool, error) {
	result := r.db.WithContext(ctx).Where("file_id = ? AND status = ?", fileId, fileentity.FileStatusDeleting).Delete(&model.File{})
	return result.RowsAffected == 1, result.Error
}
