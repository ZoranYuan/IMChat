package file

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func FileKey(fileId string) string {
	return cachekey.FileMeta(fileId)
}

func MultipartMetaKey(uploadId string) string {
	return "im:file:multipart:" + uploadId + ":meta"
}

func FileHashKey(uploaderId string, fileHash string) string {
	return "im:file:hash:" + uploaderId + ":" + fileHash
}

func MultipartInitLockKey(uploaderId string, fileHash string) string {
	return "im:file:multipart:init-lock:" + uploaderId + ":" + fileHash
}

func FileDedupInitLockKey(uploaderId string, fileHash string) string {
	return "im:file:dedup:init-lock:" + uploaderId + ":" + fileHash
}

func MultipartCompleteLockKey(uploadId string) string {
	return "im:file:multipart:complete-lock:" + uploadId
}

func ActiveUploadKey(uploaderId string, fileHash string) string {
	return "im:file:multipart:hash:" + uploaderId + ":" + fileHash
}
