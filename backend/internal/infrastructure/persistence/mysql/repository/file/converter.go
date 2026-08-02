package file

import (
	fileentity "IM_backend/internal/domain/file/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func toFileModel(f *fileentity.File) *model.File {
	var fileHash *string
	if f.FileHash != "" {
		fileHash = &f.FileHash
	}
	return &model.File{
		FileId:      f.FileId,
		UploaderId:  f.UploaderId,
		Bucket:      f.Bucket,
		ObjectKey:   f.ObjectKey,
		FileName:    f.FileName,
		ContentType: f.ContentType,
		FileHash:    fileHash,
		Size:        f.Size,
		CreatedAt:   f.CreatedAt,
	}
}

func toFileDomain(m *model.File) *fileentity.File {
	return &fileentity.File{
		FileId:      m.FileId,
		UploaderId:  m.UploaderId,
		Bucket:      m.Bucket,
		ObjectKey:   m.ObjectKey,
		FileName:    m.FileName,
		ContentType: m.ContentType,
		FileHash:    stringValue(m.FileHash),
		Size:        m.Size,
		CreatedAt:   m.CreatedAt,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
