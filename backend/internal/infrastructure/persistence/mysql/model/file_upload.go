package model

type FileUpload struct {
	UploadId        string  `gorm:"size:64;primaryKey" json:"uploadId"`
	FileId          string  `gorm:"size:32;not null;index:idx_file_upload_file" json:"fileId"`
	UploaderId      string  `gorm:"size:32;not null;index:idx_file_upload_uploader;index:idx_file_upload_hash,priority:1" json:"uploaderId"`
	UploadMode      string  `gorm:"size:16;not null;index:idx_file_upload_mode" json:"uploadMode"`
	StorageUploadId *string `gorm:"size:128;index:idx_file_upload_storage" json:"storageUploadId,omitempty"`

	FileHash     string `gorm:"size:128;index:idx_file_upload_hash,priority:2" json:"fileHash,omitempty"`
	ObjectKey    string `gorm:"size:255;not null;uniqueIndex:uk_file_upload_object" json:"objectKey"`
	FileName     string `gorm:"size:255;not null" json:"fileName"`
	ContentType  string `gorm:"size:128" json:"contentType,omitempty"`
	ExpectedSize int64  `gorm:"not null" json:"expectedSize"`

	ChunkSize   *int64 `json:"chunkSize,omitempty"`
	TotalChunks *int   `json:"totalChunks,omitempty"`

	Status      string `gorm:"size:16;not null;index:idx_file_upload_status_expire,priority:1;index:idx_file_upload_cleanup,priority:1" json:"status"`
	ExpiresAt   int64  `gorm:"not null;index:idx_file_upload_status_expire,priority:2;index:idx_file_upload_cleanup,priority:3" json:"expiresAt"`
	RetryCount  int    `gorm:"not null;default:0;index:idx_file_upload_cleanup,priority:4" json:"retryCount"`
	NextRetryAt int64  `gorm:"not null;default:0;index:idx_file_upload_cleanup,priority:2" json:"nextRetryAt"`
	LockedAt    *int64 `gorm:"index:idx_file_upload_locked" json:"lockedAt,omitempty"`
	LockToken   string `gorm:"size:64;not null;default:'';index:idx_file_upload_lock_token" json:"-"`
	LastError   string `gorm:"type:text" json:"lastError,omitempty"`
	CreatedAt   int64  `gorm:"not null;index:idx_file_upload_created" json:"createdAt"`
	UpdatedAt   int64  `gorm:"not null" json:"updatedAt"`
	CompletedAt *int64 `json:"completedAt,omitempty"`
}

func (FileUpload) TableName() string {
	return "file_uploads"
}
