package object

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error
	CreateMultipartUpload(ctx context.Context, objectKey string, contentType string) (string, error)
	UploadMultipartPart(ctx context.Context, objectKey string, uploadId string, partNumber int, reader io.Reader, size int64) (string, error)
	CompleteMultipartUpload(ctx context.Context, objectKey string, uploadId string, parts []MultipartPart) error
	AbortMultipartUpload(ctx context.Context, objectKey string, uploadId string) error
	PresignedGetURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error)
	Bucket() string
}

type MultipartPart struct {
	PartNumber int
	ETag       string
}
