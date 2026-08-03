package file

import (
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MultipartUploadRepository struct {
	db *gorm.DB
}

func NewMultipartUploadRepository(db *gorm.DB) filerepo.MultipartUploadRepository {
	return &MultipartUploadRepository{db: db}
}

func (m *MultipartUploadRepository) WithTx(tx any) filerepo.MultipartUploadRepository {
	return &MultipartUploadRepository{db: tx.(*gorm.DB)}
}

func (r *MultipartUploadRepository) Create(ctx context.Context, upload filerepo.MultipartUploadRecord) error {
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

func (r *MultipartUploadRepository) ClaimExpired(ctx context.Context, now int64, staleBefore int64, limit int) ([]filerepo.MultipartUploadRecord, error) {
	if limit <= 0 {
		return []filerepo.MultipartUploadRecord{}, nil
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
		return []filerepo.MultipartUploadRecord{}, nil
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

	uploads := make([]filerepo.MultipartUploadRecord, 0, len(rows))
	for i := range rows {
		row := rows[i]
		row.LockedAt = &now
		row.LockToken = lockToken
		uploads = append(uploads, toMultipartUploadRecord(row))
	}
	return uploads, nil
}

func (r *MultipartUploadRepository) MarkExpired(ctx context.Context, uploadId, lockToken string, updatedAt int64) (bool, error) {
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

func (r *MultipartUploadRepository) MarkCleanupRetry(ctx context.Context, uploadId, lockToken string, nextRetryAt int64, lastError string, updatedAt int64) (bool, error) {
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

func (r *MultipartUploadRepository) MarkCleanupFailed(ctx context.Context, uploadId, lockToken string, lastError string, updatedAt int64) (bool, error) {
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

func (r *MultipartUploadRepository) MarkCompleted(ctx context.Context, uploadId string, completedAt int64) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.FileUpload{}).
		Where("upload_id = ? AND status = ?", uploadId, "uploading").
		Updates(map[string]any{
			"status":       "completed",
			"completed_at": completedAt,
			"updated_at":   completedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, nil
	}
	var status string
	if err := r.db.WithContext(ctx).
		Model(&model.FileUpload{}).
		Select("status").
		Where("upload_id = ?", uploadId).
		Scan(&status).Error; err != nil {
		return false, err
	}
	return status == "completed", nil
}

func (r *MultipartUploadRepository) Delete(ctx context.Context, uploadId string) error {
	return r.db.WithContext(ctx).Where("upload_id = ?", uploadId).Delete(&model.FileUpload{}).Error
}

func toMultipartUploadRecord(row model.FileUpload) filerepo.MultipartUploadRecord {
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

	return filerepo.MultipartUploadRecord{
		UploadId:        row.UploadId,
		FileId:          row.FileId,
		UploaderId:      row.UploaderId,
		UploadMode:      row.UploadMode,
		StorageUploadId: storageUploadID,
		FileHash:        row.FileHash,
		ObjectKey:       row.ObjectKey,
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
