package file

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func FileKey(fileId string) string {
	return cachekey.FileMeta(fileId)
}

func MultipartMetaKey(uploadId string) string {
	return "im:file:multipart:" + uploadId + ":meta"
}

func MultipartPartsKey(uploadId string) string {
	return "im:file:multipart:" + uploadId + ":parts"
}

func FileHashKey(fileHash string) string {
	return "im:file:hash:" + fileHash
}

func ActiveUploadKey(fileHash string) string {
	return "im:file:multipart:hash:" + fileHash
}
