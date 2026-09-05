package file

import fileapp "IM_backend/internal/application/file"

type FileResponse struct {
	FileID      string `json:"fileId"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	CreatedAt   int64  `json:"createdAt"`
}

type AttachmentAccessURLResponse struct {
	AttachmentID string `json:"attachmentId"`
	FileName     string `json:"fileName"`
	ContentType  string `json:"contentType"`
	Size         int64  `json:"size"`
	MediaURL     string `json:"mediaUrl"`
	ThumbURL     string `json:"thumbUrl,omitempty"`
	ExpiresAt    int64  `json:"expiresAt"`
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

type DirectUploadInitResponse struct {
	UploadID  string `json:"uploadId"`
	FileID    string `json:"fileId"`
	Status    string `json:"status"`
	URL       string `json:"url,omitempty"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
}

type MultipartInitResponse struct {
	UploadID      string `json:"uploadId"`
	FileID        string `json:"fileId"`
	Status        string `json:"status"`
	UploadedParts []int  `json:"uploadedParts"`
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
		FileID:      dto.FileID,
		FileName:    dto.FileName,
		ContentType: dto.ContentType,
		Size:        dto.Size,
		CreatedAt:   dto.CreatedAt,
	}
}

func toAttachmentAccessURLResponse(dto *fileapp.AttachmentAccessURLDTO) AttachmentAccessURLResponse {
	return AttachmentAccessURLResponse{
		AttachmentID: dto.AttachmentID,
		FileName:     dto.FileName,
		ContentType:  dto.ContentType,
		Size:         dto.Size,
		MediaURL:     dto.MediaURL,
		ThumbURL:     dto.ThumbURL,
		ExpiresAt:    dto.ExpiresAt,
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
		UploadedParts: dto.UploadedParts,
	}
}

func toDirectUploadInitResponse(dto *fileapp.DirectUploadInitResDTO) DirectUploadInitResponse {
	return DirectUploadInitResponse{
		UploadID:  dto.UploadID,
		FileID:    dto.FileID,
		Status:    dto.Status,
		URL:       dto.URL,
		ExpiresAt: dto.ExpiresAt,
	}
}
