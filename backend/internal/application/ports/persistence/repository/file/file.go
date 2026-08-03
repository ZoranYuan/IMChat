package file

import (
	fileentity "IM_backend/internal/domain/file/entity"
	"context"
)

type FileRepository interface {
	Save(ctx context.Context, file *fileentity.File) error
	GetByID(ctx context.Context, fileId string) (*fileentity.File, error)
	FindByUploaderAndHash(ctx context.Context, uploaderId string, fileHash string) (*fileentity.File, error)
	BatchGetByIDs(ctx context.Context, fileIds []string) (map[string]*fileentity.File, error)
	ListOrphanCandidates(ctx context.Context, before int64, limit int) ([]*fileentity.File, error)
	MarkDeleting(ctx context.Context, fileId string, before int64) (bool, error)
	DeleteDeleting(ctx context.Context, fileId string) (bool, error)
	WithTx(tx any) FileRepository
}

type FileUploadRepository interface {
	Create(ctx context.Context, upload FileUploadRecord) error
	GetByID(ctx context.Context, uploadId string) (*FileUploadRecord, error)
	FindUploadingByUploaderAndHash(ctx context.Context, uploaderId, fileHash string, now int64) (*FileUploadRecord, error)
	ClaimExpired(ctx context.Context, now int64, staleBefore int64, limit int) ([]FileUploadRecord, error)
	MarkExpired(ctx context.Context, uploadId, lockToken string, updatedAt int64) (bool, error)
	MarkCleanupRetry(ctx context.Context, uploadId, lockToken string, nextRetryAt int64, lastError string, updatedAt int64) (bool, error)
	MarkCleanupFailed(ctx context.Context, uploadId, lockToken string, lastError string, updatedAt int64) (bool, error)
	MarkCompleted(ctx context.Context, uploadId, fileId string, completedAt int64) (bool, error)
	Delete(ctx context.Context, uploadId string) error
	WithTx(tx any) FileUploadRepository
}

type FileUploadRecord struct {
	UploadId        string
	FileId          string
	UploaderId      string
	UploadMode      string
	StorageUploadId string
	FileHash        string
	ObjectKey       string
	FileName        string
	ContentType     string
	ExpectedSize    int64
	ChunkSize       int64
	TotalChunks     int
	Status          string
	ExpiresAt       int64
	RetryCount      int
	NextRetryAt     int64
	LockedAt        *int64
	LockToken       string
	LastError       string
	CreatedAt       int64
	UpdatedAt       int64
	CompletedAt     *int64
}
