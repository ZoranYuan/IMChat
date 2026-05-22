package file

import "io"

type UploadDTO struct {
	UploaderId  string
	FileName    string
	ContentType string
	Size        int64
	Reader      io.Reader
}

type MultipartInitDTO struct {
	UploaderId  string
	FileName    string
	ContentType string
	Size        int64
	FileHash    string
	ChunkSize   int64
	TotalChunks int
}

type MultipartInitResDTO struct {
	UploadId      string   `json:"uploadId"`
	FileId        string   `json:"fileId"`
	ObjectKey     string   `json:"objectKey"`
	UploadedParts []int    `json:"uploadedParts"`
	Completed     bool     `json:"completed"`
	File          *FileDTO `json:"file,omitempty"`
}

type MultipartPartDTO struct {
	UploadId   string
	UploaderId string
	PartNumber int
	ChunkHash  string
	Size       int64
	Reader     io.Reader
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
