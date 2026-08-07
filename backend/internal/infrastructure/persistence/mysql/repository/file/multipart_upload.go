package file

import (
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FileUploadRepository struct {
	db *gorm.DB
}

func NewFileUploadRepository(db *gorm.DB) filerepo.FileUploadRepository {
	return &FileUploadRepository{db: db}
}

func (r *FileUploadRepository) WithTx(tx any) filerepo.FileUploadRepository {
	return &FileUploadRepository{db: tx.(*gorm.DB)}
}

func (r *FileUploadRepository) Create(ctx context.Context, upload filerepo.FileUploadRecord) error {
	var storageUploadID *string
	if upload.StorageUploadId != "" {
		storageUploadID = &upload.StorageUploadId
	}
	var chunkSize *int64
	if upload.ChunkSize > 0 {
		chunkSize = &upload.ChunkSize
	}
	var totalChunks *int
	if upload.TotalChunks > 0 {
		totalChunks = &upload.TotalChunks
	}
	return r.db.WithContext(ctx).Create(&model.FileUpload{
		UploadId:        upload.UploadId,
		FileId:          upload.FileId,
		UploaderId:      upload.UploaderId,
		UploadMode:      upload.UploadMode,
		StorageUploadId: storageUploadID,
		FileHash:        upload.FileHash,
		ObjectKey:       upload.ObjectKey,
		FileName:        upload.FileName,
		ContentType:     upload.ContentType,
		ExpectedSize:    upload.ExpectedSize,
		ChunkSize:       chunkSize,
		TotalChunks:     totalChunks,
		Status:          upload.Status,
		ExpiresAt:       upload.ExpiresAt,
		RetryCount:      upload.RetryCount,
		NextRetryAt:     upload.NextRetryAt,
		LockedAt:        upload.LockedAt,
		LockToken:       upload.LockToken,
		LastError:       upload.LastError,
		CreatedAt:       upload.CreatedAt,
		UpdatedAt:       upload.UpdatedAt,
	}).Error
}

func (r *FileUploadRepository) GetByID(ctx context.Context, uploadId string) (*filerepo.FileUploadRecord, error) {
	var row model.FileUpload
	if err := r.db.WithContext(ctx).Where("upload_id = ?", uploadId).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	record := toFileUploadRecord(row)
	return &record, nil
}

func (r *FileUploadRepository) FindUploadingByUploaderAndHash(ctx context.Context, uploaderId, fileHash string, now int64) (*filerepo.FileUploadRecord, error) {
	var row model.FileUpload
	err := r.db.WithContext(ctx).
		Where("uploader_id = ? AND file_hash = ? AND status = ? AND expires_at > ?", uploaderId, fileHash, "uploading", now).
		Order("created_at DESC").
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	record := toFileUploadRecord(row)
	return &record, nil
}

func (r *FileUploadRepository) ClaimExpired(ctx context.Context, now int64, staleBefore int64, limit int) ([]filerepo.FileUploadRecord, error) {
	if limit <= 0 {
		return []filerepo.FileUploadRecord{}, nil
	}

	var rows []model.FileUpload
	query := r.db.WithContext(ctx).
		Where("status = ? AND expires_at <= ? AND next_retry_at <= ?", "uploading", now, now).
		Where("locked_at IS NULL OR locked_at <= ?", staleBefore).
		Order("retry_count ASC, expires_at ASC, upload_id ASC").
		Limit(limit).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})

	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []filerepo.FileUploadRecord{}, nil
	}

	ids := make([]string, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].UploadId)
	}
	lockToken := uuid.NewString()
	if err := r.db.WithContext(ctx).
		Model(&model.FileUpload{}).
		Where("upload_id IN ?", ids).
		Updates(map[string]any{
			"locked_at":  now,
			"lock_token": lockToken,
			"updated_at": now,
		}).Error; err != nil {
		return nil, err
	}

	uploads := make([]filerepo.FileUploadRecord, 0, len(rows))
	for i := range rows {
		row := rows[i]
		row.LockedAt = &now
		row.LockToken = lockToken
		uploads = append(uploads, toFileUploadRecord(row))
	}
	return uploads, nil
}

func (r *FileUploadRepository) MarkExpired(ctx context.Context, uploadId, lockToken string, updatedAt int64) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.FileUpload{}).
		Where("upload_id = ? AND status = ? AND lock_token = ?", uploadId, "uploading", lockToken).
		Updates(map[string]any{
			"status":     "expired",
			"locked_at":  nil,
			"lock_token": "",
			"updated_at": updatedAt,
		})
	return result.RowsAffected > 0, result.Error
}

func (r *FileUploadRepository) MarkCleanupRetry(ctx context.Context, uploadId, lockToken string, nextRetryAt int64, lastError string, updatedAt int64) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.FileUpload{}).
		Where("upload_id = ? AND status = ? AND lock_token = ?", uploadId, "uploading", lockToken).
		Updates(map[string]any{
			"retry_count":   gorm.Expr("retry_count + 1"),
			"next_retry_at": nextRetryAt,
			"locked_at":     nil,
			"lock_token":    "",
			"last_error":    lastError,
			"updated_at":    updatedAt,
		})
	return result.RowsAffected > 0, result.Error
}

func (r *FileUploadRepository) MarkCleanupFailed(ctx context.Context, uploadId, lockToken string, lastError string, updatedAt int64) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.FileUpload{}).
		Where("upload_id = ? AND status = ? AND lock_token = ?", uploadId, "uploading", lockToken).
		Updates(map[string]any{
			"status":     "cleanup_failed",
			"locked_at":  nil,
			"lock_token": "",
			"last_error": lastError,
			"updated_at": updatedAt,
		})
	return result.RowsAffected > 0, result.Error
}

func (r *FileUploadRepository) MarkCompleted(ctx context.Context, uploadId, fileId string, completedAt int64) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.FileUpload{}).
		Where("upload_id = ? AND status = ?", uploadId, "uploading").
		Updates(map[string]any{
			"status":       "completed",
			"file_id":      fileId,
			"completed_at": completedAt,
			"updated_at":   completedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, nil
	}
	var row struct {
		Status string
		FileID string `gorm:"column:file_id"`
	}
	if err := r.db.WithContext(ctx).
		Model(&model.FileUpload{}).
		Select("status, file_id").
		Where("upload_id = ?", uploadId).
		Scan(&row).Error; err != nil {
		return false, err
	}
	return row.Status == "completed" && row.FileID == fileId, nil
}

func (r *FileUploadRepository) Delete(ctx context.Context, uploadId string) error {
	return r.db.WithContext(ctx).Where("upload_id = ?", uploadId).Delete(&model.FileUpload{}).Error
}

func toFileUploadRecord(row model.FileUpload) filerepo.FileUploadRecord {
	chunkSize := int64(0)
	if row.ChunkSize != nil {
		chunkSize = *row.ChunkSize
	}
	totalChunks := 0
	if row.TotalChunks != nil {
		totalChunks = *row.TotalChunks
	}
	storageUploadID := ""
	if row.StorageUploadId != nil {
		storageUploadID = *row.StorageUploadId
	}

	return filerepo.FileUploadRecord{
		UploadId:        row.UploadId,
		FileId:          row.FileId,
		UploaderId:      row.UploaderId,
		UploadMode:      row.UploadMode,
		StorageUploadId: storageUploadID,
		FileHash:        row.FileHash,
		ObjectKey:       row.ObjectKey,
		FileName:        row.FileName,
		ContentType:     row.ContentType,
		ExpectedSize:    row.ExpectedSize,
		ChunkSize:       chunkSize,
		TotalChunks:     totalChunks,
		Status:          row.Status,
		ExpiresAt:       row.ExpiresAt,
		RetryCount:      row.RetryCount,
		NextRetryAt:     row.NextRetryAt,
		LockedAt:        row.LockedAt,
		LockToken:       row.LockToken,
		LastError:       row.LastError,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		CompletedAt:     row.CompletedAt,
	}
}
