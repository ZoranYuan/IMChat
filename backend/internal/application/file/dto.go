package file

import "io"

type UploadDTO struct {
	UploaderId  string
	FileName    string
	ContentType string
	Size        int64
	Reader      io.Reader
}

type FileDTO struct {
	FileId      string `json:"fileId"`
	UploaderId  string `json:"uploaderId"`
	Bucket      string `json:"bucket"`
	ObjectKey   string `json:"objectKey"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
	CreatedAt   int64  `json:"createdAt"`
}
