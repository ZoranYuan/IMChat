package object

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error
	PresignedGetURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error)
	Bucket() string
}
