package file

import (
	fileentity "IM_backend/internal/domain/file/entity"
	"io"
)

func toDTO(file *fileentity.File) *FileDTO {
	return &FileDTO{
		FileId:      file.FileId,
		UploaderId:  file.UploaderId,
		FileName:    file.FileName,
		ContentType: file.ContentType,
		Size:        file.Size,
		CreatedAt:   file.CreatedAt,
	}
}

type UploadDTO struct {
	UploaderId  string
	FileName    string
	ContentType string
	Size        int64
	FileHash    string
	Reader      io.Reader
}

type DirectUploadInitDTO struct {
	UploaderId  string
	FileName    string
	ContentType string
	Size        int64
	FileHash    string
}

type DirectUploadInitResDTO struct {
	UploadId  string `json:"uploadId"`
	FileId    string `json:"fileId"`
	Status    string `json:"status"`
	URL       string `json:"url,omitempty"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
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
	UploadId      string `json:"uploadId"`
	FileId        string `json:"fileId"`
	Status        string `json:"status"`
	UploadedParts []int  `json:"uploadedParts"`
}

type MultipartPartURLDTO struct {
	UploadId   string `json:"uploadId"`
	PartNumber int    `json:"partNumber"`
	URL        string `json:"url"`
}

type AttachmentAccessURLDTO struct {
	AttachmentId string `json:"attachmentId"`
	FileName     string `json:"fileName"`
	ContentType  string `json:"contentType"`
	Size         int64  `json:"size"`
	URL          string `json:"url"`
	ExpiresAt    int64  `json:"expiresAt"`
}

type FileDTO struct {
	FileId      string `json:"fileId"`
	UploaderId  string `json:"uploaderId"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	CreatedAt   int64  `json:"createdAt"`
}
