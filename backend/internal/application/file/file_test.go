package file

import (
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	objectstorage "IM_backend/internal/application/ports/storage/object"
	fileentity "IM_backend/internal/domain/file/entity"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type fakeIDGenerator struct {
	id string
}

func (g fakeIDGenerator) Generate() (string, error) {
	if g.id == "" {
		return "10001", nil
	}
	return g.id, nil
}

type fakeFileRepository struct {
	saved *fileentity.File
	files map[string]*fileentity.File
	err   error
}

func (r *fakeFileRepository) Save(ctx context.Context, file *fileentity.File) error {
	if r.err != nil {
		return r.err
	}
	r.saved = file
	if r.files == nil {
		r.files = make(map[string]*fileentity.File)
	}
	r.files[file.FileId] = file
	return nil
}

func (r *fakeFileRepository) GetByID(ctx context.Context, fileId string) (*fileentity.File, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.files[fileId], nil
}

type fakeFileCache struct {
	file   *fileentity.File
	set    *fileentity.File
	setTTL time.Duration
	err    error
}

func (c *fakeFileCache) Set(ctx context.Context, file *fileentity.File, ttl time.Duration) error {
	c.set = file
	c.setTTL = ttl
	return c.err
}

func (c *fakeFileCache) Get(ctx context.Context, fileId string) (*fileentity.File, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.file, nil
}

func (c *fakeFileCache) SetMultipartUpload(ctx context.Context, meta filecache.MultipartUploadMeta, ttl time.Duration) error {
	return c.err
}

func (c *fakeFileCache) GetMultipartUpload(ctx context.Context, uploadId string) (*filecache.MultipartUploadMeta, error) {
	return nil, c.err
}

func (c *fakeFileCache) AcquireMultipartInitLock(ctx context.Context, uploaderId string, fileHash string, ttl time.Duration) (string, bool, error) {
	return "test-lock-token", c.err == nil, c.err
}

func (c *fakeFileCache) ReleaseMultipartInitLock(ctx context.Context, uploaderId string, fileHash string, token string) error {
	return c.err
}

func (c *fakeFileCache) AcquireMultipartCompleteLock(ctx context.Context, uploadId string, ttl time.Duration) (string, bool, error) {
	return "test-lock-token", c.err == nil, c.err
}

func (c *fakeFileCache) ReleaseMultipartCompleteLock(ctx context.Context, uploadId string, token string) error {
	return c.err
}

func (c *fakeFileCache) DeleteMultipartUpload(ctx context.Context, uploadId string) error {
	return c.err
}

func (c *fakeFileCache) SetActiveUpload(ctx context.Context, uploaderId string, fileHash string, uploadId string, ttl time.Duration) error {
	return c.err
}

func (c *fakeFileCache) GetActiveUploadId(ctx context.Context, uploaderId string, fileHash string) (string, error) {
	return "", c.err
}

func (c *fakeFileCache) DeleteActiveUpload(ctx context.Context, uploaderId string, fileHash string) error {
	return c.err
}

func (c *fakeFileCache) SetFileHash(ctx context.Context, fileHash string, fileId string, ttl time.Duration) error {
	return c.err
}

func (c *fakeFileCache) GetFileIdByHash(ctx context.Context, fileHash string) (string, error) {
	return "", c.err
}

type fakeObjectStorage struct {
	bucket     string
	putKey     string
	putSize    int64
	putType    string
	putContent string
	uploadId   string
	presignKey string
	presignTTL time.Duration
	putErr     error
	presignErr error
}

func (s *fakeObjectStorage) PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	if s.putErr != nil {
		return s.putErr
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.putKey = objectKey
	s.putSize = size
	s.putType = contentType
	s.putContent = string(data)
	return nil
}

func (s *fakeObjectStorage) CreateMultipartUpload(ctx context.Context, objectKey string, contentType string) (string, error) {
	if s.uploadId == "" {
		s.uploadId = "upload-1"
	}
	return s.uploadId, s.putErr
}

func (s *fakeObjectStorage) PresignMultipartPart(ctx context.Context, objectKey string, uploadId string, partNumber int, ttl time.Duration) (string, error) {
	return "http://minio.test/" + objectKey, s.putErr
}

func (s *fakeObjectStorage) ListMultipartParts(ctx context.Context, objectKey string, uploadId string) ([]objectstorage.MultipartPart, error) {
	return nil, s.putErr
}

func (s *fakeObjectStorage) CompleteMultipartUpload(ctx context.Context, objectKey string, uploadId string, parts []objectstorage.MultipartPart) error {
	if s.putErr != nil {
		return s.putErr
	}
	s.putKey = objectKey
	return nil
}

func (s *fakeObjectStorage) AbortMultipartUpload(ctx context.Context, objectKey string, uploadId string) error {
	return nil
}

func (s *fakeObjectStorage) PresignedGetURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	if s.presignErr != nil {
		return "", s.presignErr
	}
	s.presignKey = objectKey
	s.presignTTL = ttl
	return "https://minio.local/" + objectKey, nil
}

func (s *fakeObjectStorage) Bucket() string {
	return s.bucket
}

func testOptions() Options {
	return Options{
		CacheTTL: 30 * time.Second,
		URLTTL:   time.Minute,
	}
}

func TestUploadStoresObjectMetadataAndCache(t *testing.T) {
	repo := &fakeFileRepository{}
	cache := &fakeFileCache{}
	storage := &fakeObjectStorage{bucket: "videos"}
	app := NewFileApplication(testOptions(), repo, cache, storage, fakeIDGenerator{})

	dto, err := app.Upload(context.Background(), UploadDTO{
		UploaderId:  "u1",
		FileName:    "movie.mp4",
		ContentType: "video/mp4",
		Size:        int64(len("video-bytes")),
		Reader:      bytes.NewBufferString("video-bytes"),
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	if repo.saved == nil {
		t.Fatalf("expected file metadata to be saved")
	}
	if cache.set == nil {
		t.Fatalf("expected file metadata to be cached")
	}
	if cache.setTTL != 30*time.Second {
		t.Fatalf("cache ttl = %v, want 30s", cache.setTTL)
	}
	if storage.presignTTL != 60*time.Second {
		t.Fatalf("url ttl = %v, want 60s", storage.presignTTL)
	}
	if storage.putContent != "video-bytes" || storage.putSize != int64(len("video-bytes")) {
		t.Fatalf("unexpected uploaded object content=%q size=%d", storage.putContent, storage.putSize)
	}
	if !strings.HasPrefix(dto.ObjectKey, "uploads/") || !strings.HasSuffix(dto.ObjectKey, "/"+dto.FileId+".mp4") {
		t.Fatalf("unexpected object key: %q", dto.ObjectKey)
	}
	if dto.URL != "https://minio.local/"+dto.ObjectKey {
		t.Fatalf("unexpected url: %q", dto.URL)
	}
	if dto.Bucket != "videos" || dto.UploaderId != "u1" {
		t.Fatalf("unexpected file dto: %+v", dto)
	}
}

func TestUploadRejectsEmptyFile(t *testing.T) {
	app := NewFileApplication(testOptions(), &fakeFileRepository{}, &fakeFileCache{}, &fakeObjectStorage{}, fakeIDGenerator{})

	_, err := app.Upload(context.Background(), UploadDTO{
		UploaderId: "u1",
		FileName:   "empty.mp4",
		Size:       0,
		Reader:     bytes.NewReader(nil),
	})
	if !errors.Is(err, ErrFileRequired) {
		t.Fatalf("Upload() error = %v, want %v", err, ErrFileRequired)
	}
}

func TestGetUsesCacheAndRefreshesURL(t *testing.T) {
	cached := fileentity.NewFile(
		"file-1",
		"u1",
		"videos",
		"uploads/20260519/u1/file-1.mp4",
		"movie.mp4",
		"video/mp4",
		100,
		"stale-url",
	)
	cache := &fakeFileCache{file: cached}
	storage := &fakeObjectStorage{bucket: "videos"}
	app := NewFileApplication(testOptions(), &fakeFileRepository{}, cache, storage, fakeIDGenerator{})

	dto, err := app.Get(context.Background(), "file-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if dto.URL != "https://minio.local/"+cached.ObjectKey {
		t.Fatalf("expected refreshed presigned url, got %q", dto.URL)
	}
	if storage.presignKey != cached.ObjectKey {
		t.Fatalf("presign key = %q, want %q", storage.presignKey, cached.ObjectKey)
	}
}

func TestGetLoadsRepositoryOnCacheMiss(t *testing.T) {
	stored := fileentity.NewFile(
		"file-1",
		"u1",
		"videos",
		"uploads/20260519/u1/file-1.mp4",
		"movie.mp4",
		"video/mp4",
		100,
		"",
	)
	repo := &fakeFileRepository{files: map[string]*fileentity.File{"file-1": stored}}
	cache := &fakeFileCache{}
	storage := &fakeObjectStorage{bucket: "videos"}
	app := NewFileApplication(testOptions(), repo, cache, storage, fakeIDGenerator{})

	dto, err := app.Get(context.Background(), "file-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if dto.FileId != "file-1" || cache.set == nil {
		t.Fatalf("expected repository result to be returned and cached, dto=%+v cache=%+v", dto, cache.set)
	}
}

func TestBuildCompletePartsReturnsMissingParts(t *testing.T) {
	app := NewFileApplication(testOptions(), &fakeFileRepository{}, &fakeFileCache{}, &fakeObjectStorage{}, fakeIDGenerator{})
	meta := &filecache.MultipartUploadMeta{
		Size:        15,
		ChunkSize:   5,
		TotalChunks: 3,
	}

	_, err := app.buildCompleteParts(meta, []objectstorage.MultipartPart{
		{PartNumber: 1, ETag: "etag-1", Size: 5},
		{PartNumber: 3, ETag: "etag-3", Size: 5},
	})

	var incompleteErr *UploadIncompleteError
	if !errors.As(err, &incompleteErr) {
		t.Fatalf("expected UploadIncompleteError, got %v", err)
	}
	if len(incompleteErr.MissingParts) != 1 || incompleteErr.MissingParts[0] != 2 {
		t.Fatalf("missing parts = %+v, want [2]", incompleteErr.MissingParts)
	}
}

func TestBuildCompletePartsReturnsInvalidParts(t *testing.T) {
	app := NewFileApplication(testOptions(), &fakeFileRepository{}, &fakeFileCache{}, &fakeObjectStorage{}, fakeIDGenerator{})
	meta := &filecache.MultipartUploadMeta{
		Size:        15,
		ChunkSize:   5,
		TotalChunks: 3,
	}

	_, err := app.buildCompleteParts(meta, []objectstorage.MultipartPart{
		{PartNumber: 1, ETag: "etag-1", Size: 5},
		{PartNumber: 2, ETag: "etag-2", Size: 4},
		{PartNumber: 3, ETag: "etag-3", Size: 5},
	})

	var incompleteErr *UploadIncompleteError
	if !errors.As(err, &incompleteErr) {
		t.Fatalf("expected UploadIncompleteError, got %v", err)
	}
	if len(incompleteErr.InvalidParts) != 1 || incompleteErr.InvalidParts[0] != 2 {
		t.Fatalf("invalid parts = %+v, want [2]", incompleteErr.InvalidParts)
	}
}

func TestBuildCompletePartsSortsPartsForStorage(t *testing.T) {
	app := NewFileApplication(testOptions(), &fakeFileRepository{}, &fakeFileCache{}, &fakeObjectStorage{}, fakeIDGenerator{})
	meta := &filecache.MultipartUploadMeta{
		Size:        15,
		ChunkSize:   5,
		TotalChunks: 3,
	}

	parts, err := app.buildCompleteParts(meta, []objectstorage.MultipartPart{
		{PartNumber: 3, ETag: "etag-3", Size: 5},
		{PartNumber: 1, ETag: "etag-1", Size: 5},
		{PartNumber: 2, ETag: "etag-2", Size: 5},
	})
	if err != nil {
		t.Fatalf("buildCompleteParts() error = %v", err)
	}
	for index, part := range parts {
		if part.PartNumber != index+1 {
			t.Fatalf("part at index %d = %d, want %d", index, part.PartNumber, index+1)
		}
	}
}
