package model

type File struct {
	FileId      string `gorm:"size:32;primaryKey" json:"fileId"`
	UploaderId  string `gorm:"size:32;not null;index:idx_file_uploader" json:"uploaderId"`
	Bucket      string `gorm:"size:128;not null" json:"bucket"`
	ObjectKey   string `gorm:"size:255;not null;uniqueIndex" json:"objectKey"`
	FileName    string `gorm:"size:255;not null" json:"fileName"`
	ContentType string `gorm:"size:128" json:"contentType"`
	Size        int64  `gorm:"not null" json:"size"`
	URL         string `gorm:"size:512" json:"url"`
	CreatedAt   int64  `gorm:"not null;index:idx_file_created_at" json:"createdAt"`
}
