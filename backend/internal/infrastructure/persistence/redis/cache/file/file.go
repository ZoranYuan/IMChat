package file

import (
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	fileentity "IM_backend/internal/domain/file/entity"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
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
