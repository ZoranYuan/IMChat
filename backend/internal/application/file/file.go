package file

import (
	idport "IM_backend/internal/application/ports/id"
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	objectstorage "IM_backend/internal/application/ports/storage/object"
	fileentity "IM_backend/internal/domain/file/entity"
	"context"
	"log"
	"path"
	"sort"
	"strings"
	"time"
)

type FileApplication struct {
	options        Options
	fileRepository filerepo.FileRepository
	fileCache      filecache.FileCache
	storage        objectstorage.ObjectStorage
	idGenerator    idport.Generator
}

type Options struct {
	MultipartTTL             time.Duration
	PartURLTTL               time.Duration
	MultipartInitLockTTL     time.Duration
	MultipartCompleteLockTTL time.Duration
	CacheTTL                 time.Duration
	URLTTL                   time.Duration
}

const (
	multipartStatusUploading = "uploading"
	multipartStatusCompleted = "completed"
)

func NewFileApplication(
	options Options,
	fileRepository filerepo.FileRepository,
	fileCache filecache.FileCache,
	storage objectstorage.ObjectStorage,
	idGenerator idport.Generator,
) *FileApplication {
	return &FileApplication{
		options:        options,
		fileRepository: fileRepository,
		fileCache:      fileCache,
		storage:        storage,
		idGenerator:    idGenerator,
	}
}

func (a *FileApplication) Upload(ctx context.Context, dto UploadDTO) (*FileDTO, error) {
	if dto.Reader == nil || dto.Size <= 0 {
		return nil, ErrFileRequired
	}

	fileId, err := a.idGenerator.Generate()
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

	if err := a.fileRepository.Save(ctx, file); err != nil {
		return nil, err
	}

	_ = a.fileCache.Set(ctx, file, a.cacheTTL())

	return toDTO(file), nil
}

func (a *FileApplication) InitMultipartUpload(ctx context.Context, dto MultipartInitDTO) (*MultipartInitResDTO, error) {
	if dto.UploaderId == "" || dto.FileName == "" || dto.Size <= 0 || dto.ChunkSize <= 0 || dto.TotalChunks <= 0 || dto.FileHash == "" {
		return nil, ErrInvalidPart
	}
	expectedChunks := int((dto.Size + dto.ChunkSize - 1) / dto.ChunkSize)
	if dto.TotalChunks != expectedChunks {
		return nil, ErrInvalidPart
	}
	lockToken, locked, err := a.fileCache.AcquireMultipartInitLock(ctx, dto.UploaderId, dto.FileHash, a.multipartInitLockTTL())
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, ErrUploadBusy
	}
	defer func() {
		_ = a.fileCache.ReleaseMultipartInitLock(context.Background(), dto.UploaderId, dto.FileHash, lockToken)
	}()

	// 文件成功上传后，会记录 hash 值，以此来实现后续的秒传
	if fileId, err := a.fileCache.GetFileIdByHash(ctx, dto.FileHash); err != nil {
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

	// 文件未上传完整
	if uploadId, err := a.fileCache.GetActiveUploadId(ctx, dto.UploaderId, dto.FileHash); err != nil {
		return nil, err
	} else if uploadId != "" {
		meta, err := a.fileCache.GetMultipartUpload(ctx, uploadId)
		if err != nil {
			return nil, err
		}
		if meta != nil && meta.UploaderId == dto.UploaderId && meta.FileHash == dto.FileHash {
			// 获取所有已经上传的切片索引
			uploadedParts, err := a.uploadedPartNumbers(ctx, meta.ObjectKey, uploadId)
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

	// 新的文件上传
	fileId, err := a.idGenerator.Generate()
	if err != nil {
		return nil, err
	}
	objectKey := a.buildObjectKey(dto.UploaderId, fileId, dto.FileName)
	uploadId, err := a.storage.CreateMultipartUpload(ctx, objectKey, dto.ContentType)
	if err != nil {
		return nil, err
	}

	meta := filecache.MultipartUploadMeta{
		UploadId:    uploadId,
		FileId:      fileId,
		UploaderId:  dto.UploaderId,
		Bucket:      a.storage.Bucket(),
		ObjectKey:   objectKey,
		FileName:    dto.FileName,
		ContentType: dto.ContentType,
		Size:        dto.Size,
		FileHash:    dto.FileHash,
		ChunkSize:   dto.ChunkSize,
		TotalChunks: dto.TotalChunks,
		CreatedAt:   time.Now().UnixMilli(),
		Status:      multipartStatusUploading,
	}

	// 保存上传任务元数据
	if err := a.fileCache.SetMultipartUpload(ctx, meta, a.multipartTTL()); err != nil {
		_ = a.storage.AbortMultipartUpload(ctx, meta.ObjectKey, uploadId)
		return nil, err
	}

	// 防止同一个文件被重复初始化上传
	if err := a.fileCache.SetActiveUpload(ctx, dto.UploaderId, dto.FileHash, uploadId, a.multipartTTL()); err != nil {
		_ = a.storage.AbortMultipartUpload(ctx, meta.ObjectKey, uploadId)
		_ = a.fileCache.DeleteMultipartUpload(ctx, uploadId)
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

func (a *FileApplication) PresignMultipartParts(ctx context.Context, uploadId string, uploaderId string, partNumbers []int) ([]MultipartPartURLDTO, error) {
	meta, err := a.fileCache.GetMultipartUpload(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, ErrInvalidUpload
	}
	if meta.UploaderId != uploaderId {
		return nil, ErrUploadUnauthorized
	}
	if len(partNumbers) == 0 || len(partNumbers) > 1000 {
		return nil, ErrInvalidPart
	}
	seen := make(map[int]struct{}, len(partNumbers))
	result := make([]MultipartPartURLDTO, 0, len(partNumbers))
	for _, partNumber := range partNumbers {
		if partNumber <= 0 || partNumber > meta.TotalChunks {
			return nil, ErrInvalidPart
		}
		if _, ok := seen[partNumber]; ok {
			continue
		}
		seen[partNumber] = struct{}{}
		url, err := a.storage.PresignMultipartPart(ctx, meta.ObjectKey, meta.UploadId, partNumber, a.partURLTTL())
		if err != nil {
			return nil, err
		}
		result = append(result, MultipartPartURLDTO{UploadId: uploadId, PartNumber: partNumber, URL: url})
	}
	return result, nil
}

func (a *FileApplication) CompleteMultipartUpload(ctx context.Context, uploadId string, uploaderId string) (*FileDTO, error) {
	meta, err := a.fileCache.GetMultipartUpload(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, ErrInvalidUpload
	}
	if meta.UploaderId != uploaderId {
		return nil, ErrUploadUnauthorized
	}
	if meta.Status == multipartStatusCompleted {
		return a.Get(ctx, meta.FileId)
	}
	lockToken, locked, err := a.fileCache.AcquireMultipartCompleteLock(ctx, uploadId, a.multipartCompleteLockTTL())
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, ErrUploadBusy
	}
	defer func() {
		_ = a.fileCache.ReleaseMultipartCompleteLock(context.Background(), uploadId, lockToken)
	}()

	meta, err = a.fileCache.GetMultipartUpload(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, ErrInvalidUpload
	}
	if meta.Status == multipartStatusCompleted {
		return a.Get(ctx, meta.FileId)
	}

	// 数据库幂等兜底，确保 DB 才是文件元数据事实源
	if existing, err := a.fileRepository.GetByID(ctx, meta.FileId); err != nil {
		return nil, err
	} else if existing != nil {
		meta.Status = multipartStatusCompleted
		if err := a.fileCache.SetMultipartUpload(ctx, *meta, a.multipartTTL()); err != nil {
			log.Printf("修复已完成上传任务缓存失败：上传=%s 错误=%v", meta.UploadId, err)
		}
		return a.Get(ctx, meta.FileId)
	}

	parts, err := a.storage.ListMultipartParts(ctx, meta.ObjectKey, uploadId)
	if err != nil {
		return nil, err
	}

	completeParts, err := a.buildCompleteParts(meta, parts)
	if err != nil {
		return nil, err
	}

	if err := a.storage.CompleteMultipartUpload(ctx, meta.ObjectKey, meta.UploadId, completeParts); err != nil {
		return nil, err
	}
	url, err := a.storage.PresignedGetURL(ctx, meta.ObjectKey, a.urlTTL())
	if err != nil {
		return nil, err
	}

	entity := fileentity.NewFile(meta.FileId, meta.UploaderId, meta.Bucket, meta.ObjectKey, meta.FileName, meta.ContentType, meta.Size, url)
	if err := a.fileRepository.Save(ctx, entity); err != nil {
		return nil, err
	}
	if err := a.fileCache.Set(ctx, entity, a.cacheTTL()); err != nil {
		log.Printf("完成分片上传后写入文件缓存失败：文件=%s 错误=%v", entity.FileId, err)
	}
	// 文件上传成功后，在 redis 中记录已经上传成功的 文件的总 hash, 用于实现秒传
	if err := a.fileCache.SetFileHash(ctx, meta.FileHash, meta.FileId, a.multipartTTL()); err != nil {
		log.Printf("完成分片上传后写入文件哈希缓存失败：文件=%s 错误=%v", meta.FileId, err)
	}
	meta.Status = multipartStatusCompleted
	if err := a.fileCache.SetMultipartUpload(ctx, *meta, a.multipartTTL()); err != nil {
		log.Printf("完成分片上传后更新上传任务缓存失败：上传=%s 错误=%v", meta.UploadId, err)
	}
	if err := a.fileCache.DeleteActiveUpload(ctx, meta.UploaderId, meta.FileHash); err != nil {
		log.Printf("完成分片上传后删除活跃上传缓存失败：上传=%s 错误=%v", meta.UploadId, err)
	}

	return toDTO(entity), nil
}

func (a *FileApplication) buildCompleteParts(meta *filecache.MultipartUploadMeta, parts []objectstorage.MultipartPart) ([]objectstorage.MultipartPart, error) {
	partByNumber := make(map[int]objectstorage.MultipartPart, len(parts))
	invalidParts := make([]int, 0)
	for _, part := range parts {
		if part.PartNumber <= 0 || part.PartNumber > meta.TotalChunks || part.ETag == "" || part.Size != expectedMultipartPartSize(meta, part.PartNumber) {
			invalidParts = append(invalidParts, part.PartNumber)
			continue
		}
		if _, exists := partByNumber[part.PartNumber]; exists {
			invalidParts = append(invalidParts, part.PartNumber)
			continue
		}
		partByNumber[part.PartNumber] = part
	}

	missingParts := make([]int, 0)
	for partNumber := 1; partNumber <= meta.TotalChunks; partNumber++ {
		if _, exists := partByNumber[partNumber]; !exists {
			missingParts = append(missingParts, partNumber)
		}
	}

	if len(missingParts) > 0 || len(invalidParts) > 0 {
		return nil, &UploadIncompleteError{
			MissingParts: missingParts,
			InvalidParts: invalidParts,
		}
	}

	completeParts := make([]objectstorage.MultipartPart, 0, meta.TotalChunks)
	for partNumber := 1; partNumber <= meta.TotalChunks; partNumber++ {
		part := partByNumber[partNumber]
		completeParts = append(completeParts, objectstorage.MultipartPart{PartNumber: partNumber, ETag: part.ETag})
	}
	return completeParts, nil
}

func expectedMultipartPartSize(meta *filecache.MultipartUploadMeta, partNumber int) int64 {
	if partNumber == meta.TotalChunks {
		return meta.Size - int64(meta.TotalChunks-1)*meta.ChunkSize
	}
	return meta.ChunkSize
}

func (a *FileApplication) uploadedPartNumbers(ctx context.Context, objectKey string, uploadId string) ([]int, error) {
	parts, err := a.storage.ListMultipartParts(ctx, objectKey, uploadId)
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

func (a *FileApplication) Get(ctx context.Context, fileId string) (*FileDTO, error) {
	file, err := a.fileCache.Get(ctx, fileId)
	if err != nil {
		return nil, err
	}
	if file != nil {
		if url, err := a.storage.PresignedGetURL(ctx, file.ObjectKey, a.urlTTL()); err == nil {
			file.URL = url
		}
		return toDTO(file), nil
	}

	file, err = a.fileRepository.GetByID(ctx, fileId)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, ErrFileNotFound
	}

	if url, err := a.storage.PresignedGetURL(ctx, file.ObjectKey, a.urlTTL()); err == nil {
		file.URL = url
	}
	_ = a.fileCache.Set(ctx, file, a.cacheTTL())

	return toDTO(file), nil
}

func (a *FileApplication) buildObjectKey(uploaderId string, fileId string, fileName string) string {
	ext := path.Ext(fileName)
	day := time.Now().Format("20060102")
	uploaderId = strings.TrimSpace(uploaderId)
	if uploaderId == "" {
		uploaderId = "unknown"
	}
	return "uploads/" + day + "/" + uploaderId + "/" + fileId + ext
}

func (a *FileApplication) multipartInitLockTTL() time.Duration {
	if a.options.MultipartInitLockTTL <= 0 {
		return 15 * time.Second
	}
	return a.options.MultipartInitLockTTL
}

func (a *FileApplication) multipartCompleteLockTTL() time.Duration {
	if a.options.MultipartCompleteLockTTL <= 0 {
		return 30 * time.Second
	}
	return a.options.MultipartCompleteLockTTL
}

func (a *FileApplication) partURLTTL() time.Duration {
	if a.options.PartURLTTL <= 0 {
		return 15 * time.Minute
	}
	return a.options.PartURLTTL
}

func (a *FileApplication) multipartTTL() time.Duration {
	ttl := a.options.MultipartTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return ttl
}

func (a *FileApplication) cacheTTL() time.Duration {
	ttl := a.options.CacheTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return ttl
}

func (a *FileApplication) urlTTL() time.Duration {
	ttl := a.options.URLTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	return ttl
}
