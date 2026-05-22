package file

import (
	fileentity "IM_backend/internal/domain/file/entity"
	"context"
	"time"
)

type MultipartUploadMeta struct {
	UploadId    string `json:"uploadId"`
	FileId      string `json:"fileId"`
	UploaderId  string `json:"uploaderId"`
	Bucket      string `json:"bucket"`
	ObjectKey   string `json:"objectKey"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	FileHash    string `json:"fileHash"`
	ChunkSize   int64  `json:"chunkSize"`
	TotalChunks int    `json:"totalChunks"`
	CreatedAt   int64  `json:"createdAt"`
}

type MultipartUploadPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
	ChunkHash  string `json:"chunkHash"`
}

type FileCache interface {
	Set(ctx context.Context, file *fileentity.File, ttl time.Duration) error
	Get(ctx context.Context, fileId string) (*fileentity.File, error)
	SetMultipartUpload(ctx context.Context, meta MultipartUploadMeta, ttl time.Duration) error
	GetMultipartUpload(ctx context.Context, uploadId string) (*MultipartUploadMeta, error)
	AddMultipartPart(ctx context.Context, uploadId string, part MultipartUploadPart, ttl time.Duration) error
	ListMultipartParts(ctx context.Context, uploadId string) ([]MultipartUploadPart, error)
	DeleteMultipartUpload(ctx context.Context, uploadId string) error
	SetActiveUpload(ctx context.Context, fileHash string, uploadId string, ttl time.Duration) error
	GetActiveUploadId(ctx context.Context, fileHash string) (string, error)
	DeleteActiveUpload(ctx context.Context, fileHash string) error
	SetFileHash(ctx context.Context, fileHash string, fileId string, ttl time.Duration) error
	GetFileIdByHash(ctx context.Context, fileHash string) (string, error)
}
