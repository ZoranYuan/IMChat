package file

import fileapp "IM_backend/internal/application/file"

type FileRes struct {
	FileId      string `json:"fileId"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	CreatedAt   int64  `json:"createdAt"`
}

type AttachmentAccessURLRes struct {
	AttachmentId string `json:"attachmentId"`
	FileName     string `json:"fileName"`
	ContentType  string `json:"contentType"`
	Size         int64  `json:"size"`
	URL          string `json:"url"`
	ExpiresAt    int64  `json:"expiresAt"`
}

type MultipartInitReq struct {
	FileName    string `json:"fileName" binding:"required"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size" binding:"required"`
	FileHash    string `json:"fileHash" binding:"required"`
	ChunkSize   int64  `json:"chunkSize" binding:"required"`
	TotalChunks int    `json:"totalChunks" binding:"required"`
}

type MultipartInitRes struct {
	UploadId      string   `json:"uploadId"`
	FileId        string   `json:"fileId"`
	Status        string   `json:"status"`
	UploadedParts []int    `json:"uploadedParts"`
}

type MultipartPartsPresignReq struct {
	PartNumbers []int `json:"partNumbers" binding:"required"`
}

type MultipartPartURLRes struct {
	UploadId   string `json:"uploadId"`
	PartNumber int    `json:"partNumber"`
	URL        string `json:"url"`
}

func toFileRes(dto *fileapp.FileDTO) FileRes {
	return FileRes{
		FileId:      dto.FileId,
		FileName:    dto.FileName,
		ContentType: dto.ContentType,
		Size:        dto.Size,
		CreatedAt:   dto.CreatedAt,
	}
}

func toAttachmentAccessURLRes(dto *fileapp.AttachmentAccessURLDTO) AttachmentAccessURLRes {
	return AttachmentAccessURLRes{
		AttachmentId: dto.AttachmentId,
		FileName:     dto.FileName,
		ContentType:  dto.ContentType,
		Size:         dto.Size,
		URL:          dto.URL,
		ExpiresAt:    dto.ExpiresAt,
	}
}

func toMultipartInitRes(dto *fileapp.MultipartInitResDTO) MultipartInitRes {
	return MultipartInitRes{
		UploadId:      dto.UploadId,
		FileId:        dto.FileId,
		Status:        dto.Status,
		UploadedParts: dto.UploadedParts,
	}
}
