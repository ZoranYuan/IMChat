package file

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func FileKey(fileId string) string {
	return cachekey.FileMeta(fileId)
}
