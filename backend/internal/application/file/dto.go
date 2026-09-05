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

type DirectUploadInitResDTO struct {
	UploadID  string
	FileID    string
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
	FileID        string
	Status        string
	UploadedParts []int
}

type MultipartPartURLDTO struct {
	UploadID   string
	PartNumber int
	URL        string
}

type AttachmentAccessURLDTO struct {
	AttachmentID string
	FileName     string
	ContentType  string
	Size         int64
	MediaURL     string
	ThumbURL     string
	ExpiresAt    int64
}

type FileDTO struct {
	FileID      string
	UploaderID  string
	FileName    string
	ContentType string
	Size        int64
	CreatedAt   int64
}
