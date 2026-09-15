package file

import (
	fileentity "IM_backend/internal/domain/file/entity"
	"context"
	"time"
)

// UploadMeta 保存直传和分片上传共用的上传元数据。
// 直传时 StorageUploadId、ChunkSize、TotalChunks 为空；分片上传时会填充这些字段。
type UploadMeta struct {
	UploadId        string `json:"uploadId"`
	UploadMode      string `json:"uploadMode"`
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
	ExpiresAt       int64  `json:"expiresAt"`
	Status          string `json:"status"`
}

// AttachmentFileCard 是消息附件的稳定信息，缓存时间可以长于访问 URL。
// ObjectKey 只供后端生成预签名 URL，不能直接返回给前端。
type AttachmentFileCard struct {
	AttachmentID       string `json:"attachmentId"`
	FileID             string `json:"fileId"`
	ObjectKey          string `json:"objectKey"`
	FileName           string `json:"fileName"`
	ContentType        string `json:"contentType"`
	Size               int64  `json:"size"`
	CType              int    `json:"cType"`
	Width              int    `json:"width,omitempty"`
	Height             int    `json:"height,omitempty"`
	DurationMs         *int64 `json:"durationMs,omitempty"`
	AttachmentExpireAt int64  `json:"attachmentExpireAt"`
}

// AttachmentURL 是短期的对象访问地址，TTL 必须小于 URL 自身的有效期。
type AttachmentURL struct {
	AttachmentID string `json:"attachmentId"`
	MediaURL     string `json:"mediaUrl"`
	ThumbURL     string `json:"thumbUrl,omitempty"`
	ExpiresAt    int64  `json:"expiresAt"`
}

type FileCache interface {
	SetFileMetadata(ctx context.Context, file *fileentity.File, ttl time.Duration) error
	GetFileMetadata(ctx context.Context, fileID string) (*fileentity.File, error)
	DeleteFileMetadata(ctx context.Context, fileID string) error
	SetFileByUploaderAndHash(ctx context.Context, uploaderID string, fileHash string, file *fileentity.File, ttl time.Duration) error
	GetFileByUploaderAndHash(ctx context.Context, uploaderID string, fileHash string) (*fileentity.File, error)
	DeleteFileByUploaderAndHash(ctx context.Context, uploaderID string, fileHash string) error
	SetUploadMeta(ctx context.Context, meta UploadMeta, ttl time.Duration) error
	GetUploadMeta(ctx context.Context, uploadID string) (*UploadMeta, error)
	DeleteUploadMeta(ctx context.Context, uploadID string) error
	AcquireFileInitLock(ctx context.Context, uploaderID string, fileHash string, ttl time.Duration) (string, bool, error)
	ReleaseFileInitLock(ctx context.Context, uploaderID string, fileHash string, token string) error
	AcquireFileCompleteLock(ctx context.Context, uploadID string, ttl time.Duration) (string, bool, error)
	ReleaseFileCompleteLock(ctx context.Context, uploadID string, token string) error
	SetActiveFileUpload(ctx context.Context, uploaderID string, fileHash string, uploadID string, ttl time.Duration) error
	GetActiveFileUploadID(ctx context.Context, uploaderID string, fileHash string) (string, error)
	DeleteActiveFileUploadIfMatches(ctx context.Context, uploaderID string, fileHash string, uploadID string) error
	RenewFileCompleteLock(ctx context.Context, uploadID string, token string, ttl time.Duration) (bool, error)
	GetAttachmentFileCardBatch(ctx context.Context, attachmentIDs []string) (map[string]*AttachmentFileCard, error)
	SetAttachmentFileCardBatch(ctx context.Context, values []*AttachmentFileCard, ttl time.Duration) error
	DeleteAttachmentFileCard(ctx context.Context, attachmentIDs []string) error
	GetAttachmentURLBatch(ctx context.Context, attachmentIDs []string) (map[string]*AttachmentURL, error)
	SetAttachmentURLBatch(ctx context.Context, values []*AttachmentURL, ttl time.Duration) error
	DeleteAttachmentURL(ctx context.Context, attachmentIDs []string) error
}
