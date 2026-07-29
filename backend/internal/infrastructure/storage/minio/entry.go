package minio

import (
	"IM_backend/configs"
	objectport "IM_backend/internal/application/ports/storage/object"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ObjectStorage struct {
	client         *minio.Client
	core           *minio.Core
	publicClient   *minio.Client
	publicCore     *minio.Core
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
	publicCore := &minio.Core{Client: publicClient}

	return &ObjectStorage{
		client:         client,
		core:           core,
		publicClient:   publicClient,
		publicCore:     publicCore,
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

func (s *ObjectStorage) PresignMultipartPart(ctx context.Context, objectKey string, uploadId string, partNumber int, ttl time.Duration) (string, error) {
	if uploadId == "" || partNumber <= 0 {
		return "", fmt.Errorf("参数错误")
	}
	u, err := s.publicCore.Presign(ctx, http.MethodPut, s.bucket, objectKey, ttl, url.Values{
		"uploadId":   []string{uploadId},
		"partNumber": []string{strconv.Itoa(partNumber)},
	})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *ObjectStorage) ListMultipartParts(ctx context.Context, objectKey string, uploadId string) ([]objectport.MultipartPart, error) {
	marker := 0
	result := make([]objectport.MultipartPart, 0)
	for {
		listed, err := s.core.ListObjectParts(ctx, s.bucket, objectKey, uploadId, marker, 1000)
		if err != nil {
			return nil, err
		}
		for _, part := range listed.ObjectParts {
			result = append(result, objectport.MultipartPart{PartNumber: part.PartNumber, ETag: part.ETag, Size: part.Size})
		}
		if !listed.IsTruncated {
			return result, nil
		}
		marker = listed.NextPartNumberMarker
	}
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
