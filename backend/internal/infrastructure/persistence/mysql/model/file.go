package model

type File struct {
	FileId      string  `gorm:"size:32;primaryKey" json:"fileId"`
	UploaderId  string  `gorm:"size:32;not null;index:idx_file_uploader;index:uk_file_uploader_hash,priority:1" json:"uploaderId"`
	Bucket      string  `gorm:"size:128;not null" json:"bucket"`
	ObjectKey   string  `gorm:"size:255;not null;uniqueIndex" json:"objectKey"`
	FileName    string  `gorm:"size:255;not null" json:"fileName"`
	ContentType string  `gorm:"size:128" json:"contentType"`
	FileHash    *string `gorm:"size:128;uniqueIndex:uk_file_uploader_hash,priority:2" json:"fileHash,omitempty"`
	Size        int64   `gorm:"not null" json:"size"`
	CreatedAt   int64   `gorm:"not null;index:idx_file_created_at" json:"createdAt"`
	Status      string  `gorm:"size:16;not null;default:uploaded;index:idx_file_status_created" json:"status"`
}
