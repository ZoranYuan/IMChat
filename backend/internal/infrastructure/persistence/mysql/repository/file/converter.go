package file

import (
	fileentity "IM_backend/internal/domain/file/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func toFileModel(f *fileentity.File) *model.File {
	return &model.File{
		FileId:      f.FileId,
		UploaderId:  f.UploaderId,
		Bucket:      f.Bucket,
		ObjectKey:   f.ObjectKey,
		FileName:    f.FileName,
		ContentType: f.ContentType,
		Size:        f.Size,
		URL:         f.URL,
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
		Size:        m.Size,
		URL:         m.URL,
		CreatedAt:   m.CreatedAt,
	}
}
