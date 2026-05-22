package file

import (
	"IM_backend/configs"
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	objectstorage "IM_backend/internal/application/ports/storage/object"
	fileentity "IM_backend/internal/domain/file/entity"
	"IM_backend/internal/infrastructure/id/snow"
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
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

func (a *Application) InitMultipartUpload(ctx context.Context, dto MultipartInitDTO) (*MultipartInitResDTO, error) {
	if dto.UploaderId == "" || dto.FileName == "" || dto.Size <= 0 || dto.ChunkSize <= 0 || dto.TotalChunks <= 0 || dto.FileHash == "" {
		return nil, ErrInvalidPart
	}

	if fileId, err := a.cache.GetFileIdByHash(ctx, dto.FileHash); err != nil {
		return nil, err
	} else if fileId != "" {
		file, err := a.Get(ctx, fileId)
		if err == nil && file != nil {
			return &MultipartInitResDTO{
				FileId:    file.FileId,
				ObjectKey: file.ObjectKey,
				Completed: true,
				File:      file,
			}, nil
		}
	}

	if uploadId, err := a.cache.GetActiveUploadId(ctx, dto.FileHash); err != nil {
		return nil, err
	} else if uploadId != "" {
		meta, err := a.cache.GetMultipartUpload(ctx, uploadId)
		if err != nil {
			return nil, err
		}
		if meta != nil && meta.UploaderId == dto.UploaderId && meta.FileHash == dto.FileHash {
			uploadedParts, err := a.uploadedPartNumbers(ctx, uploadId)
			if err != nil {
				return nil, err
			}
			return &MultipartInitResDTO{
				UploadId:      uploadId,
				FileId:        meta.FileId,
				ObjectKey:     meta.ObjectKey,
				UploadedParts: uploadedParts,
				Completed:     false,
			}, nil
		}
	}

	fileId, err := snow.GenerateSnowID(int(a.config.App.MachineID))
	if err != nil {
		return nil, err
	}
	uploadId, err := snow.GenerateSnowID(int(a.config.App.MachineID))
	if err != nil {
		return nil, err
	}

	meta := filecache.MultipartUploadMeta{
		UploadId:    uploadId,
		FileId:      fileId,
		UploaderId:  dto.UploaderId,
		Bucket:      a.storage.Bucket(),
		ObjectKey:   a.buildObjectKey(dto.UploaderId, fileId, dto.FileName),
		FileName:    dto.FileName,
		ContentType: dto.ContentType,
		Size:        dto.Size,
		FileHash:    dto.FileHash,
		ChunkSize:   dto.ChunkSize,
		TotalChunks: dto.TotalChunks,
		CreatedAt:   time.Now().UnixMilli(),
	}

	if err := a.cache.SetMultipartUpload(ctx, meta, a.multipartTTL()); err != nil {
		return nil, err
	}
	if err := a.cache.SetActiveUpload(ctx, dto.FileHash, uploadId, a.multipartTTL()); err != nil {
		return nil, err
	}

	return &MultipartInitResDTO{
		UploadId:      uploadId,
		FileId:        fileId,
		ObjectKey:     meta.ObjectKey,
		UploadedParts: []int{},
		Completed:     false,
	}, nil
}

func (a *Application) UploadMultipartPart(ctx context.Context, dto MultipartPartDTO) ([]int, error) {
	meta, err := a.cache.GetMultipartUpload(ctx, dto.UploadId)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, ErrInvalidUpload
	}
	if meta.UploaderId != dto.UploaderId {
		return nil, ErrUploadUnauthorized
	}
	if dto.Reader == nil || dto.PartNumber <= 0 || dto.PartNumber > meta.TotalChunks || dto.Size <= 0 {
		return nil, ErrInvalidPart
	}

	partPath := a.multipartPartPath(meta.UploadId, dto.PartNumber)
	if err := os.MkdirAll(filepath.Dir(partPath), 0755); err != nil {
		return nil, err
	}
	partFile, err := os.Create(partPath)
	if err != nil {
		return nil, err
	}
	written, copyErr := io.Copy(partFile, dto.Reader)
	closeErr := partFile.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if written != dto.Size {
		return nil, ErrInvalidPart
	}

	if err := a.cache.AddMultipartPart(ctx, meta.UploadId, filecache.MultipartUploadPart{
		PartNumber: dto.PartNumber,
		Size:       dto.Size,
		ChunkHash:  dto.ChunkHash,
	}, a.multipartTTL()); err != nil {
		return nil, err
	}

	return a.uploadedPartNumbers(ctx, meta.UploadId)
}

func (a *Application) CompleteMultipartUpload(ctx context.Context, uploadId string, uploaderId string) (*FileDTO, error) {
	meta, err := a.cache.GetMultipartUpload(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, ErrInvalidUpload
	}
	if meta.UploaderId != uploaderId {
		return nil, ErrUploadUnauthorized
	}

	parts, err := a.cache.ListMultipartParts(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	if len(parts) != meta.TotalChunks {
		return nil, ErrUploadNotCompleted
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	for i, part := range parts {
		if part.PartNumber != i+1 {
			return nil, ErrUploadNotCompleted
		}
	}

	mergedPath := a.multipartMergedPath(uploadId)
	if err := os.MkdirAll(filepath.Dir(mergedPath), 0755); err != nil {
		return nil, err
	}
	merged, err := os.Create(mergedPath)
	if err != nil {
		return nil, err
	}
	for _, part := range parts {
		partFile, err := os.Open(a.multipartPartPath(uploadId, part.PartNumber))
		if err != nil {
			_ = merged.Close()
			return nil, err
		}
		_, copyErr := io.Copy(merged, partFile)
		closeErr := partFile.Close()
		if copyErr != nil {
			_ = merged.Close()
			return nil, copyErr
		}
		if closeErr != nil {
			_ = merged.Close()
			return nil, closeErr
		}
	}
	if err := merged.Close(); err != nil {
		return nil, err
	}

	file, err := os.Open(mergedPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if err := a.storage.PutObject(ctx, meta.ObjectKey, file, meta.Size, meta.ContentType); err != nil {
		return nil, err
	}
	url, err := a.storage.PresignedGetURL(ctx, meta.ObjectKey, a.urlTTL())
	if err != nil {
		return nil, err
	}

	entity := fileentity.NewFile(meta.FileId, meta.UploaderId, meta.Bucket, meta.ObjectKey, meta.FileName, meta.ContentType, meta.Size, url)
	if err := a.repository.Save(ctx, entity); err != nil {
		return nil, err
	}
	_ = a.cache.Set(ctx, entity, a.cacheTTL())
	_ = a.cache.SetFileHash(ctx, meta.FileHash, meta.FileId, a.multipartTTL())
	_ = a.cache.DeleteMultipartUpload(ctx, uploadId)
	_ = a.cache.DeleteActiveUpload(ctx, meta.FileHash)
	_ = os.RemoveAll(a.multipartRoot(uploadId))

	return toDTO(entity), nil
}

func (a *Application) uploadedPartNumbers(ctx context.Context, uploadId string) ([]int, error) {
	parts, err := a.cache.ListMultipartParts(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	res := make([]int, 0, len(parts))
	for _, part := range parts {
		res = append(res, part.PartNumber)
	}
	sort.Ints(res)
	return res, nil
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

func (a *Application) multipartTTL() time.Duration {
	return 24 * time.Hour
}

func (a *Application) multipartRoot(uploadId string) string {
	return filepath.Join(os.TempDir(), "im_uploads", uploadId)
}

func (a *Application) multipartPartPath(uploadId string, partNumber int) string {
	return filepath.Join(a.multipartRoot(uploadId), "parts", strconv.Itoa(partNumber))
}

func (a *Application) multipartMergedPath(uploadId string) string {
	return filepath.Join(a.multipartRoot(uploadId), "merged")
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
