package file

import fileapp "IM_backend/internal/application/file"

type FileResponse struct {
	Status       string `json:"status"`
	UploadID     string `json:"uploadId,omitempty"`
	FileID       string `json:"fileId,omitempty"`
	FileName     string `json:"fileName,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	Size         int64  `json:"size,omitempty"`
	CreatedAt    int64  `json:"createdAt,omitempty"`
	MissingParts []int  `json:"missingParts,omitempty"`
	InvalidParts []int  `json:"invalidParts,omitempty"`
}

type AttachmentAccessURLResponse struct {
	AttachmentID string `json:"attachmentId"`
	FileID       string `json:"fileId"`
	FileName     string `json:"fileName"`
	ContentType  string `json:"contentType"`
	Size         int64  `json:"size"`
	MediaURL     string `json:"mediaUrl"`
	ThumbURL     string `json:"thumbUrl,omitempty"`
	ExpiresAt    int64  `json:"expiresAt"`
	CType        int    `json:"cType"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	DurationMs   *int64 `json:"durationMs,omitempty"`
}

type AttachmentAccessURLsRequest struct {
	AttachmentIDs []string `json:"attachmentIds" binding:"required,min=1,max=100"`
}

type AttachmentAccessURLsResponse struct {
	Attachments []AttachmentAccessURLResponse `json:"attachments"`
}

type MultipartInitRequest struct {
	FileName    string `json:"fileName" binding:"required"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size" binding:"required"`
	FileHash    string `json:"fileHash" binding:"required"`
	ChunkSize   int64  `json:"chunkSize" binding:"required"`
	TotalChunks int    `json:"totalChunks" binding:"required"`
}

type DirectUploadInitRequest struct {
	FileName    string `json:"fileName" binding:"required"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size" binding:"required"`
	FileHash    string `json:"fileHash" binding:"required"`
}

type UploadInitRequest struct {
	FileName    string `json:"fileName" binding:"required"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size" binding:"required"`
	FileHash    string `json:"fileHash" binding:"required"`
	ChunkSize   int64  `json:"chunkSize,omitempty"`
	TotalChunks int    `json:"totalChunks,omitempty"`
}

type DirectUploadInitResponse struct {
	UploadID  string `json:"uploadId"`
	FileID    string `json:"fileId,omitempty"`
	Status    string `json:"status"`
	URL       string `json:"url,omitempty"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
}

type MultipartInitResponse struct {
	UploadID      string `json:"uploadId"`
	FileID        string `json:"fileId,omitempty"`
	Status        string `json:"status"`
	ChunkSize     int64  `json:"chunkSize,omitempty"`
	TotalChunks   int    `json:"totalChunks,omitempty"`
	UploadedParts []int  `json:"uploadedParts"`
}

type UploadInitResponse struct {
	Status        string `json:"status"`
	UploadMode    string `json:"uploadMode,omitempty"`
	UploadID      string `json:"uploadId,omitempty"`
	FileID        string `json:"fileId,omitempty"`
	URL           string `json:"url,omitempty"`
	ExpiresAt     int64  `json:"expiresAt,omitempty"`
	ChunkSize     int64  `json:"chunkSize,omitempty"`
	TotalChunks   int    `json:"totalChunks,omitempty"`
	UploadedParts []int  `json:"uploadedParts,omitempty"`
}

type MultipartPartsPresignRequest struct {
	PartNumbers []int `json:"partNumbers" binding:"required"`
}

type MultipartPartURLResponse struct {
	UploadID   string `json:"uploadId"`
	PartNumber int    `json:"partNumber"`
	URL        string `json:"url"`
}

func toFileResponse(dto *fileapp.FileDTO) FileResponse {
	return FileResponse{
		Status:      "completed",
		FileID:      dto.FileID,
		FileName:    dto.FileName,
		ContentType: dto.ContentType,
		Size:        dto.Size,
		CreatedAt:   dto.CreatedAt,
	}
}

func toUploadIncompleteResponse(uploadID string, incompleteErr *fileapp.UploadIncompleteError) FileResponse {
	return FileResponse{
		Status:       "uploading",
		UploadID:     uploadID,
		MissingParts: incompleteErr.MissingParts,
		InvalidParts: incompleteErr.InvalidParts,
	}
}

func toAttachmentAccessURLResponse(dto *fileapp.AttachmentAccessURLDTO) AttachmentAccessURLResponse {
	return AttachmentAccessURLResponse{
		AttachmentID: dto.AttachmentID,
		FileID:       dto.FileID,
		FileName:     dto.FileName,
		ContentType:  dto.ContentType,
		Size:         dto.Size,
		MediaURL:     dto.MediaURL,
		ThumbURL:     dto.ThumbURL,
		ExpiresAt:    dto.ExpiresAt,
		CType:        dto.CType,
		Width:        dto.Width,
		Height:       dto.Height,
		DurationMs:   dto.DurationMs,
	}
}

func toAttachmentAccessURLsResponse(dtos []*fileapp.AttachmentAccessURLDTO) AttachmentAccessURLsResponse {
	items := make([]AttachmentAccessURLResponse, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		items = append(items, toAttachmentAccessURLResponse(dto))
	}
	return AttachmentAccessURLsResponse{Attachments: items}
}

func toMultipartInitResponse(dto *fileapp.MultipartInitResDTO) MultipartInitResponse {
	return MultipartInitResponse{
		UploadID:      dto.UploadID,
		FileID:        dto.FileID,
		Status:        dto.Status,
		ChunkSize:     dto.ChunkSize,
		TotalChunks:   dto.TotalChunks,
		UploadedParts: dto.UploadedParts,
	}
}

func toUploadInitResponse(dto *fileapp.UploadInitResDTO) UploadInitResponse {
	return UploadInitResponse{
		Status:        dto.Status,
		UploadMode:    dto.UploadMode,
		UploadID:      dto.UploadID,
		FileID:        dto.FileID,
		URL:           dto.URL,
		ExpiresAt:     dto.ExpiresAt,
		ChunkSize:     dto.ChunkSize,
		TotalChunks:   dto.TotalChunks,
		UploadedParts: dto.UploadedParts,
	}
}

func toDirectUploadInitResponse(dto *fileapp.DirectUploadInitResDTO) DirectUploadInitResponse {
	return DirectUploadInitResponse{
		UploadID:  dto.UploadID,
		FileID:    dto.FileId,
		Status:    dto.Status,
		URL:       dto.URL,
		ExpiresAt: dto.ExpiresAt,
	}
}
