package file

import (
	"IM_backend/configs"
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	objectstorage "IM_backend/internal/application/ports/storage/object"
	fileentity "IM_backend/internal/domain/file/entity"
	"IM_backend/internal/infrastructure/id/snow"
	"context"
	"path"
	"strings"
	"time"
)

type Application struct {
	config     configs.Config
	repository filerepo.FileRepository
	cache      filecache.FileCache
	storage    objectstorage.ObjectStorage
}

func NewApplication(
	config configs.Config,
	repository filerepo.FileRepository,
	cache filecache.FileCache,
	storage objectstorage.ObjectStorage,
) *Application {
	return &Application{
		config:     config,
		repository: repository,
		cache:      cache,
		storage:    storage,
	}
}

func (a *Application) Upload(ctx context.Context, dto UploadDTO) (*FileDTO, error) {
	if dto.Reader == nil || dto.Size <= 0 {
		return nil, ErrFileRequired
	}

	fileId, err := snow.GenerateSnowID(int(a.config.App.MachineID))
	if err != nil {
		return nil, err
	}

	objectKey := a.buildObjectKey(dto.UploaderId, fileId, dto.FileName)
	if err := a.storage.PutObject(ctx, objectKey, dto.Reader, dto.Size, dto.ContentType); err != nil {
		return nil, err
	}
	url, err := a.storage.PresignedGetURL(ctx, objectKey, a.urlTTL())
	if err != nil {
		return nil, err
	}

	file := fileentity.NewFile(
		fileId,
		dto.UploaderId,
		a.storage.Bucket(),
		objectKey,
		dto.FileName,
		dto.ContentType,
		dto.Size,
		url,
	)

	if err := a.repository.Save(ctx, file); err != nil {
		return nil, err
	}

	_ = a.cache.Set(ctx, file, a.cacheTTL())

	return toDTO(file), nil
}

func (a *Application) Get(ctx context.Context, fileId string) (*FileDTO, error) {
	file, err := a.cache.Get(ctx, fileId)
	if err != nil {
		return nil, err
	}
	if file != nil {
		if url, err := a.storage.PresignedGetURL(ctx, file.ObjectKey, a.urlTTL()); err == nil {
			file.URL = url
		}
		return toDTO(file), nil
	}

	file, err = a.repository.GetByID(ctx, fileId)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, ErrFileNotFound
	}

	if url, err := a.storage.PresignedGetURL(ctx, file.ObjectKey, a.urlTTL()); err == nil {
		file.URL = url
	}
	_ = a.cache.Set(ctx, file, a.cacheTTL())

	return toDTO(file), nil
}

func (a *Application) buildObjectKey(uploaderId string, fileId string, fileName string) string {
	ext := path.Ext(fileName)
	day := time.Now().Format("20060102")
	uploaderId = strings.TrimSpace(uploaderId)
	if uploaderId == "" {
		uploaderId = "unknown"
	}
	return "uploads/" + day + "/" + uploaderId + "/" + fileId + ext
}

func (a *Application) cacheTTL() time.Duration {
	ttl := a.config.Storage.MinIO.CacheTTLSeconds
	if ttl <= 0 {
		ttl = 600
	}
	return time.Duration(ttl) * time.Second
}

func (a *Application) urlTTL() time.Duration {
	ttl := a.config.Storage.MinIO.URLTTLSeconds
	if ttl <= 0 {
		ttl = 3600
	}
	return time.Duration(ttl) * time.Second
}

func toDTO(file *fileentity.File) *FileDTO {
	return &FileDTO{
		FileId:      file.FileId,
		UploaderId:  file.UploaderId,
		Bucket:      file.Bucket,
		ObjectKey:   file.ObjectKey,
		FileName:    file.FileName,
		ContentType: file.ContentType,
		Size:        file.Size,
		URL:         file.URL,
		CreatedAt:   file.CreatedAt,
	}
}
