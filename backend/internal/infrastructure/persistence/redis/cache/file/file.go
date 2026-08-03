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

func (c *FileCache) Delete(ctx context.Context, fileId string) error {
	return c.store.Del(ctx, FileKey(fileId))
}

func (c *FileCache) RenewFileCompleteLock(ctx context.Context, uploadId string, token string, ttl time.Duration) (bool, error) {
	luaScript := `
		local token = ARGV[1]
		local ttl = ARGV[2]

		if redis.call("GET", KEYS[1]) == token then
			return redis.call("PEXPIRE", KEYS[1], ttl)
		end
		return 0
	`

	result, err := c.store.Eval(
		ctx,
		luaScript,
		[]string{FileCompleteLockKey(uploadId)},
		token,
		ttl.Milliseconds(),
	)
	if err != nil {
		return false, err
	}
	renewed, ok := result.(int64)
	return ok && renewed == 1, nil
}

func (c *FileCache) SetMultipartUploadMeta(ctx context.Context, meta filecache.MultipartUploadMeta, ttl time.Duration) error {
	return c.store.SetJSON(ctx, MultipartUploadMetaKey(meta.UploadId), meta, ttl)
}

func (c *FileCache) GetMultipartUploadMeta(ctx context.Context, uploadId string) (*filecache.MultipartUploadMeta, error) {
	var meta filecache.MultipartUploadMeta
	ok, err := c.store.GetJSON(ctx, MultipartUploadMetaKey(uploadId), &meta)
	if err != nil || !ok {
		return nil, err
	}
	return &meta, nil
}

func (c *FileCache) AcquireFileInitLock(ctx context.Context, uploaderId string, fileHash string, ttl time.Duration) (string, bool, error) {
	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}
	locked, err := c.store.SetNXString(ctx, FileInitLockKey(uploaderId, fileHash), token, ttl)
	return token, locked, err
}

func (c *FileCache) ReleaseFileInitLock(ctx context.Context, uploaderId string, fileHash string, token string) error {
	return c.releaseLock(ctx, FileInitLockKey(uploaderId, fileHash), token)
}

func (c *FileCache) AcquireFileCompleteLock(ctx context.Context, uploadId string, ttl time.Duration) (string, bool, error) {
	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}
	locked, err := c.store.SetNXString(ctx, FileCompleteLockKey(uploadId), token, ttl)
	return token, locked, err
}

func (c *FileCache) ReleaseFileCompleteLock(ctx context.Context, uploadId string, token string) error {
	return c.releaseLock(ctx, FileCompleteLockKey(uploadId), token)
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

func (c *FileCache) DeleteMultipartUploadMeta(ctx context.Context, uploadId string) error {
	return c.store.Del(ctx, MultipartUploadMetaKey(uploadId))
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

func (c *FileCache) DeleteActiveUploadIfMatches(ctx context.Context, uploaderId string, fileHash string, uploadId string) error {
	if fileHash == "" || uploadId == "" {
		return nil
	}

	const script = `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		end
		return 0
	`
	_, err := c.store.Eval(ctx, script, []string{ActiveUploadKey(uploaderId, fileHash)}, uploadId)
	return err
}

func (c *FileCache) SetFileIDByUploaderAndHash(ctx context.Context, uploaderId string, fileHash string, fileId string, ttl time.Duration) error {
	if fileHash == "" || fileId == "" || uploaderId == "" {
		return nil
	}
	return c.store.SetString(ctx, FileHashKey(uploaderId, fileHash), fileId, ttl)
}

func (c *FileCache) GetFileIDByUploaderAndHash(ctx context.Context, uploaderId string, fileHash string) (string, error) {
	if fileHash == "" || uploaderId == "" {
		return "", nil
	}
	return c.store.GetString(ctx, FileHashKey(uploaderId, fileHash))
}
