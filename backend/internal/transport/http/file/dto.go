package file

import fileapp "IM_backend/internal/application/file"

type FileRes struct {
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
	ObjectKey     string   `json:"objectKey"`
	UploadedParts []int    `json:"uploadedParts"`
	Completed     bool     `json:"completed"`
	File          *FileRes `json:"file,omitempty"`
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
		UploaderId:  dto.UploaderId,
		Bucket:      dto.Bucket,
		ObjectKey:   dto.ObjectKey,
		FileName:    dto.FileName,
		ContentType: dto.ContentType,
		Size:        dto.Size,
		URL:         dto.URL,
		CreatedAt:   dto.CreatedAt,
	}
}

func toMultipartInitRes(dto *fileapp.MultipartInitResDTO) MultipartInitRes {
	var file *FileRes
	if dto.File != nil {
		res := toFileRes(dto.File)
		file = &res
	}
	return MultipartInitRes{
		UploadId:      dto.UploadId,
		FileId:        dto.FileId,
		ObjectKey:     dto.ObjectKey,
		UploadedParts: dto.UploadedParts,
		Completed:     dto.Completed,
		File:          file,
	}
}
