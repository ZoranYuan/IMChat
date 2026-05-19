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
