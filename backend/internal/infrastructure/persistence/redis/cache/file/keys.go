package file

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func FileKey(fileId string) string {
	return cachekey.FileMeta(fileId)
}

func MultipartUploadMetaKey(uploadId string) string {
	return "im:file:multipart:" + uploadId + ":meta"
}

func FileHashKey(uploaderId string, fileHash string) string {
	return "im:file:hash:" + uploaderId + ":" + fileHash
}

func FileInitLockKey(uploaderId string, fileHash string) string {
	return "im:file:init:" + uploaderId + ":" + fileHash
}

func FileCompleteLockKey(uploadId string) string {
	return "im:file:complete:" + uploadId
}

func ActiveUploadKey(uploaderId string, fileHash string) string {
	return "im:file:multipart:hash:" + uploaderId + ":" + fileHash
}
