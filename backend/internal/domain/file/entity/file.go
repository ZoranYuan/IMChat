package entity

import "time"

type File struct {
	FileId      string
	UploaderId  string
	Bucket      string
	ObjectKey   string
	FileName    string
	ContentType string
	Size        int64
	URL         string
	CreatedAt   int64
}

func NewFile(fileId, uploaderId, bucket, objectKey, fileName, contentType string, size int64, url string) *File {
	return &File{
		FileId:      fileId,
		UploaderId:  uploaderId,
		Bucket:      bucket,
		ObjectKey:   objectKey,
		FileName:    fileName,
		ContentType: contentType,
		Size:        size,
		URL:         url,
		CreatedAt:   time.Now().UnixMilli(),
	}
}
