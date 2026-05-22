package minio

import (
	"IM_backend/configs"
	objectport "IM_backend/internal/application/ports/storage/object"
	"context"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ObjectStorage struct {
	client         *minio.Client
	core           *minio.Core
	publicClient   *minio.Client
	bucket         string
	publicEndpoint string
	useSSL         bool
}

func NewObjectStorage(ctx context.Context, cfg configs.MinIOConfig) (objectport.ObjectStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	publicEndpoint := cfg.PublicEndpoint
	if publicEndpoint == "" {
		publicEndpoint = cfg.Endpoint
	}
	publicEndpoint = strings.TrimPrefix(publicEndpoint, "http://")
	publicEndpoint = strings.TrimPrefix(publicEndpoint, "https://")

	publicClient, err := minio.New(publicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	core := &minio.Core{Client: client}

	return &ObjectStorage{
		client:         client,
		core:           core,
		publicClient:   publicClient,
		bucket:         cfg.Bucket,
		publicEndpoint: strings.TrimRight(publicEndpoint, "/"),
		useSSL:         cfg.UseSSL,
	}, nil
}

func (s *ObjectStorage) Bucket() string {
	return s.bucket
}

func (s *ObjectStorage) PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	if _, err := s.client.PutObject(ctx, s.bucket, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	}); err != nil {
		return err
	}
	return nil
}

func (s *ObjectStorage) CreateMultipartUpload(ctx context.Context, objectKey string, contentType string) (string, error) {
	return s.core.NewMultipartUpload(ctx, s.bucket, objectKey, minio.PutObjectOptions{
		ContentType: contentType,
	})
}

func (s *ObjectStorage) UploadMultipartPart(ctx context.Context, objectKey string, uploadId string, partNumber int, reader io.Reader, size int64) (string, error) {
	part, err := s.core.PutObjectPart(ctx, s.bucket, objectKey, uploadId, partNumber, reader, size, minio.PutObjectPartOptions{})
	if err != nil {
		return "", err
	}
	return part.ETag, nil
}

func (s *ObjectStorage) CompleteMultipartUpload(ctx context.Context, objectKey string, uploadId string, parts []objectport.MultipartPart) error {
	completeParts := make([]minio.CompletePart, 0, len(parts))
	for _, part := range parts {
		completeParts = append(completeParts, minio.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
	}
	_, err := s.core.CompleteMultipartUpload(ctx, s.bucket, objectKey, uploadId, completeParts, minio.PutObjectOptions{})
	return err
}

func (s *ObjectStorage) AbortMultipartUpload(ctx context.Context, objectKey string, uploadId string) error {
	return s.core.AbortMultipartUpload(ctx, s.bucket, objectKey, uploadId)
}

func (s *ObjectStorage) PresignedGetURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	u, err := s.publicClient.PresignedGetObject(ctx, s.bucket, objectKey, ttl, nil)
	usePublicEndpoint := true
	if err != nil {
		u, err = s.client.PresignedGetObject(ctx, s.bucket, objectKey, ttl, nil)
		if err != nil {
			return "", err
		}
		usePublicEndpoint = false
	}
	if !usePublicEndpoint || s.publicEndpoint == "" {
		return u.String(), nil
	}

	scheme := u.Scheme
	if s.useSSL {
		scheme = "https"
	} else {
		scheme = "http"
	}

	publicURL := *u
	publicURL.Scheme = scheme
	publicURL.Host = strings.TrimRight(s.publicEndpoint, "/")
	if _, err := url.Parse(publicURL.String()); err != nil {
		return "", err
	}
	return publicURL.String(), nil
}
