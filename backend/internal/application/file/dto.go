package file

import fileentity "IM_backend/internal/domain/file/entity"

func toFileDTO(file *fileentity.File) *FileDTO {
	return &FileDTO{
		FileID:      file.FileId,
		UploaderID:  file.UploaderId,
		FileName:    file.FileName,
		ContentType: file.ContentType,
		Size:        file.Size,
		CreatedAt:   file.CreatedAt,
	}
}

type DirectUploadInitDTO struct {
	UploaderID  string
	FileName    string
	ContentType string
	Size        int64
	FileHash    string
}

// UploadInitDTO 是统一上传入口的请求。后端根据文件大小选择直传或分片上传。
type UploadInitDTO struct {
	UploaderID  string
	FileName    string
	ContentType string
	Size        int64
	FileHash    string
	ChunkSize   int64
	TotalChunks int
}

type DirectUploadInitResDTO struct {
	UploadID  string
	FileId    string `json:"fileId,omitempty"`
	Status    string
	URL       string
	ExpiresAt int64
}

type MultipartInitDTO struct {
	UploaderID  string
	FileName    string
	ContentType string
	Size        int64
	FileHash    string
	ChunkSize   int64
	TotalChunks int
}

type MultipartInitResDTO struct {
	UploadID      string
	FileID        string `json:"fileId,omitempty"`
	Status        string
	ChunkSize     int64
	TotalChunks   int
	UploadedParts []int
}

// UploadInitResDTO 统一描述直传和分片上传的初始化结果。
// 直传使用 URL，分片上传使用 ChunkSize、TotalChunks 和 UploadedParts。
type UploadInitResDTO struct {
	UploadID      string
	FileID        string
	UploadMode    string
	Status        string
	URL           string
	ExpiresAt     int64
	ChunkSize     int64
	TotalChunks   int
	UploadedParts []int
}

type MultipartPartURLDTO struct {
	UploadID   string
	PartNumber int
	URL        string
}

type AttachmentAccessURLDTO struct {
	AttachmentID string
	FileID       string
	FileName     string
	ContentType  string
	Size         int64
	MediaURL     string
	ThumbURL     string
	ExpiresAt    int64
	CType        int
	Width        int
	Height       int
	DurationMs   *int64
}

type FileDTO struct {
	FileID      string
	UploaderID  string
	FileName    string
	ContentType string
	Size        int64
	CreatedAt   int64
}
