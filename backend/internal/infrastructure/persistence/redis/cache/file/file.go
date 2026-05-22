package file

import (
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	fileentity "IM_backend/internal/domain/file/entity"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type FileCache struct {
	store *shared.Store
}

func NewFileCache(client *redis.Client) filecache.FileCache {
	return &FileCache{store: shared.NewStore(client)}
}

func (c *FileCache) Set(ctx context.Context, file *fileentity.File, ttl time.Duration) error {
	return c.store.SetJSON(ctx, FileKey(file.FileId), file, ttl)
}

func (c *FileCache) Get(ctx context.Context, fileId string) (*fileentity.File, error) {
	var file fileentity.File
	ok, err := c.store.GetJSON(ctx, FileKey(fileId), &file)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &file, nil
}

func (c *FileCache) SetMultipartUpload(ctx context.Context, meta filecache.MultipartUploadMeta, ttl time.Duration) error {
	return c.store.SetJSON(ctx, MultipartMetaKey(meta.UploadId), meta, ttl)
}

func (c *FileCache) GetMultipartUpload(ctx context.Context, uploadId string) (*filecache.MultipartUploadMeta, error) {
	var meta filecache.MultipartUploadMeta
	ok, err := c.store.GetJSON(ctx, MultipartMetaKey(uploadId), &meta)
	if err != nil || !ok {
		return nil, err
	}
	return &meta, nil
}

func (c *FileCache) AddMultipartPart(ctx context.Context, uploadId string, part filecache.MultipartUploadPart, ttl time.Duration) error {
	data, err := json.Marshal(part)
	if err != nil {
		return err
	}
	partsKey := MultipartPartsKey(uploadId)
	metaKey := MultipartMetaKey(uploadId)
	pipe := c.store.Client().Pipeline()
	pipe.HSet(ctx, partsKey, strconv.Itoa(part.PartNumber), data)
	pipe.Expire(ctx, partsKey, ttl)
	pipe.Expire(ctx, metaKey, ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (c *FileCache) ListMultipartParts(ctx context.Context, uploadId string) ([]filecache.MultipartUploadPart, error) {
	values, err := c.store.Client().HGetAll(ctx, MultipartPartsKey(uploadId)).Result()
	if err != nil {
		return nil, err
	}
	parts := make([]filecache.MultipartUploadPart, 0, len(values))
	for _, value := range values {
		var part filecache.MultipartUploadPart
		if err := json.Unmarshal([]byte(value), &part); err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	return parts, nil
}

func (c *FileCache) DeleteMultipartUpload(ctx context.Context, uploadId string) error {
	return c.store.Del(ctx, MultipartMetaKey(uploadId), MultipartPartsKey(uploadId))
}

func (c *FileCache) SetActiveUpload(ctx context.Context, fileHash string, uploadId string, ttl time.Duration) error {
	if fileHash == "" || uploadId == "" {
		return nil
	}
	return c.store.SetString(ctx, ActiveUploadKey(fileHash), uploadId, ttl)
}

func (c *FileCache) GetActiveUploadId(ctx context.Context, fileHash string) (string, error) {
	if fileHash == "" {
		return "", nil
	}
	return c.store.GetString(ctx, ActiveUploadKey(fileHash))
}

func (c *FileCache) DeleteActiveUpload(ctx context.Context, fileHash string) error {
	if fileHash == "" {
		return nil
	}
	return c.store.Del(ctx, ActiveUploadKey(fileHash))
}

func (c *FileCache) SetFileHash(ctx context.Context, fileHash string, fileId string, ttl time.Duration) error {
	if fileHash == "" || fileId == "" {
		return nil
	}
	return c.store.SetString(ctx, FileHashKey(fileHash), fileId, ttl)
}

func (c *FileCache) GetFileIdByHash(ctx context.Context, fileHash string) (string, error) {
	if fileHash == "" {
		return "", nil
	}
	return c.store.GetString(ctx, FileHashKey(fileHash))
}
