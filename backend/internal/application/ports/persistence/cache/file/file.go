package file

import (
	fileentity "IM_backend/internal/domain/file/entity"
	"context"
	"time"
)

type MultipartUploadMeta struct {
	UploadId        string `json:"uploadId"`
	StorageUploadId string `json:"storageUploadId"`
	FileId          string `json:"fileId"`
	UploaderId      string `json:"uploaderId"`
	Bucket          string `json:"bucket"`
	ObjectKey       string `json:"objectKey"`
	FileName        string `json:"fileName"`
	ContentType     string `json:"contentType"`
	Size            int64  `json:"size"`
	FileHash        string `json:"fileHash"`
	ChunkSize       int64  `json:"chunkSize"`
	TotalChunks     int    `json:"totalChunks"`
	CreatedAt       int64  `json:"createdAt"`
	Status          string `json:"status"`
}

type FileCache interface {
	Set(ctx context.Context, file *fileentity.File, ttl time.Duration) error
	Get(ctx context.Context, fileId string) (*fileentity.File, error)
	Delete(ctx context.Context, fileId string) error
	SetMultipartUploadMeta(ctx context.Context, meta MultipartUploadMeta, ttl time.Duration) error
	GetMultipartUploadMeta(ctx context.Context, uploadId string) (*MultipartUploadMeta, error)
	DeleteMultipartUploadMeta(ctx context.Context, uploadId string) error
	AcquireFileInitLock(ctx context.Context, uploaderId string, fileHash string, ttl time.Duration) (string, bool, error)
	ReleaseFileInitLock(ctx context.Context, uploaderId string, fileHash string, token string) error
	AcquireFileCompleteLock(ctx context.Context, uploadId string, ttl time.Duration) (string, bool, error)
	ReleaseFileCompleteLock(ctx context.Context, uploadId string, token string) error
	SetActiveUpload(ctx context.Context, uploaderId string, fileHash string, uploadId string, ttl time.Duration) error
	GetActiveUploadId(ctx context.Context, uploaderId string, fileHash string) (string, error)
	DeleteActiveUpload(ctx context.Context, uploaderId string, fileHash string) error
	DeleteActiveUploadIfMatches(ctx context.Context, uploaderId string, fileHash string, uploadId string) error
	SetFileIDByUploaderAndHash(ctx context.Context, uploaderId string, fileHash string, fileId string, ttl time.Duration) error
	GetFileIDByUploaderAndHash(ctx context.Context, uploaderId string, fileHash string) (string, error)
	RenewFileCompleteLock(ctx context.Context, uploadId string, token string, ttl time.Duration) (bool, error)
}
