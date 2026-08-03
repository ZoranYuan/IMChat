package object

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	Health(ctx context.Context) error
	PresignedPutURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error)
	StatObject(ctx context.Context, objectKey string) (*ObjectInfo, error)
	OpenObject(ctx context.Context, objectKey string) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, objectKey string) error
	CreateMultipartUpload(ctx context.Context, objectKey string, contentType string) (string, error)
	PresignMultipartPart(ctx context.Context, objectKey string, uploadId string, partNumber int, ttl time.Duration) (string, error)
	ListMultipartParts(ctx context.Context, objectKey string, uploadId string) ([]MultipartPart, error)
	CompleteMultipartUpload(ctx context.Context, objectKey string, uploadId string, parts []MultipartPart) error
	AbortMultipartUpload(ctx context.Context, objectKey string, uploadId string) error
	PresignedGetURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error)
	Bucket() string
}

type ObjectInfo struct {
	Size        int64
	ContentType string
}

type MultipartPart struct {
	PartNumber int
	ETag       string
	Size       int64
}
