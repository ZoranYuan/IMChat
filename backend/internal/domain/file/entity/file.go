package entity

import "time"

type File struct {
	FileId      string
	UploaderId  string
	Bucket      string
	ObjectKey   string
	FileName    string
	ContentType string
	FileHash    string
	Size        int64
	CreatedAt   int64
}

func NewFile(fileId, uploaderId, bucket, objectKey, fileName, contentType string, size int64) *File {
	return &File{
		FileId:      fileId,
		UploaderId:  uploaderId,
		Bucket:      bucket,
		ObjectKey:   objectKey,
		FileName:    fileName,
		ContentType: contentType,
		Size:        size,
		CreatedAt:   time.Now().UnixMilli(),
	}
}
