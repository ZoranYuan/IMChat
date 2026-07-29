package file

import (
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	fileentity "IM_backend/internal/domain/file/entity"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"crypto/rand"
	"encoding/hex"
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

func (c *FileCache) AcquireMultipartInitLock(ctx context.Context, uploaderId string, fileHash string, ttl time.Duration) (string, bool, error) {
	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}
	locked, err := c.store.Client().SetNX(ctx, MultipartInitLockKey(uploaderId, fileHash), token, ttl).Result()
	return token, locked, err
}

func (c *FileCache) ReleaseMultipartInitLock(ctx context.Context, uploaderId string, fileHash string, token string) error {
	return c.releaseLock(ctx, MultipartInitLockKey(uploaderId, fileHash), token)
}

func (c *FileCache) AcquireMultipartCompleteLock(ctx context.Context, uploadId string, ttl time.Duration) (string, bool, error) {
	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}
	locked, err := c.store.Client().SetNX(ctx, MultipartCompleteLockKey(uploadId), token, ttl).Result()
	return token, locked, err
}

func (c *FileCache) ReleaseMultipartCompleteLock(ctx context.Context, uploadId string, token string) error {
	return c.releaseLock(ctx, MultipartCompleteLockKey(uploadId), token)
}

func (c *FileCache) releaseLock(ctx context.Context, key string, token string) error {
	const script = `
local value = redis.call("GET", KEYS[1])
if value == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
	return 0
`
	_, err := c.store.Client().Eval(ctx, script, []string{key}, token).Result()
	return err
}

func newLockToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func (c *FileCache) DeleteMultipartUpload(ctx context.Context, uploadId string) error {
	return c.store.Del(ctx, MultipartMetaKey(uploadId))
}

func (c *FileCache) SetActiveUpload(ctx context.Context, uploaderId string, fileHash string, uploadId string, ttl time.Duration) error {
	if fileHash == "" || uploadId == "" {
		return nil
	}
	return c.store.SetString(ctx, ActiveUploadKey(uploaderId, fileHash), uploadId, ttl)
}

func (c *FileCache) GetActiveUploadId(ctx context.Context, uploaderId string, fileHash string) (string, error) {
	if fileHash == "" {
		return "", nil
	}
	return c.store.GetString(ctx, ActiveUploadKey(uploaderId, fileHash))
}

func (c *FileCache) DeleteActiveUpload(ctx context.Context, uploaderId string, fileHash string) error {
	if fileHash == "" {
		return nil
	}
	return c.store.Del(ctx, ActiveUploadKey(uploaderId, fileHash))
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
