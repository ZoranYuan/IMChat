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

	return &ObjectStorage{
		client:         client,
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
