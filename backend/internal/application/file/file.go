package file

import (
	idport "IM_backend/internal/application/ports/id"
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"

	objectstorage "IM_backend/internal/application/ports/storage/object"
	fileentity "IM_backend/internal/domain/file/entity"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"path"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

type FileApplication struct {
	options                     Options
	fileRepository              filerepo.FileRepository
	fileUploadRepository        filerepo.FileUploadRepository
	messageAttactmentRepository messagerepo.MessageAttachmentsRepository
	fileCache                   filecache.FileCache
	storage                     objectstorage.ObjectStorage
	idGenerator                 idport.Generator
	sf                          singleflight.Group
	txManager                   txmanager.TxManager
}

type Options struct {
	MultipartTTL             time.Duration
	PartURLTTL               time.Duration
	DirectUploadURLTTL       time.Duration
	DirectUploadMaxSize      int64
	MultipartInitLockTTL     time.Duration
	MultipartCompleteLockTTL time.Duration
	DirectUploadLockTTL      time.Duration
	CacheTTL                 time.Duration
	URLTTL                   time.Duration
	MaxFileSize              int64
	MaxMultipartParts        int
}

const (
	multipartStatusUploading = "uploading"
	multipartStatusCompleted = "completed"
	multipartUploadMode      = "multipart"
	directUploadMode         = "direct"
)

func NewFileApplication(
	options Options,
	fileRepository filerepo.FileRepository,
	fileUploadRepository filerepo.FileUploadRepository,
	messageAttactmentRepository messagerepo.MessageAttachmentsRepository,
	fileCache filecache.FileCache,
	storage objectstorage.ObjectStorage,
	idGenerator idport.Generator,
	txManager txmanager.TxManager,
) *FileApplication {
	return &FileApplication{
		options:                     options,
		fileRepository:              fileRepository,
		fileUploadRepository:        fileUploadRepository,
		messageAttactmentRepository: messageAttactmentRepository,
		fileCache:                   fileCache,
		storage:                     storage,
		idGenerator:                 idGenerator,
		txManager:                   txManager,
	}
}

func normalizeFileHash(fileHash string) string {
	return strings.ToLower(strings.TrimSpace(fileHash))
}

func (a *FileApplication) InitDirectUpload(ctx context.Context, dto DirectUploadInitDTO) (*DirectUploadInitResDTO, error) {
	fileHash := normalizeFileHash(dto.FileHash)
	if dto.UploaderId == "" || dto.FileName == "" || dto.Size <= 0 || fileHash == "" {
		return nil, ErrInvalidUpload
	}
	if a.options.DirectUploadMaxSize > 0 && dto.Size > a.options.DirectUploadMaxSize {
		return nil, ErrFileTooLarge
	}
	if a.options.MaxFileSize > 0 && dto.Size > a.options.MaxFileSize {
		return nil, ErrFileTooLarge
	}

	if file, err := a.findFileByUploaderAndHash(ctx, dto.UploaderId, fileHash); err != nil {
		return nil, err
	} else if file != nil {
		return &DirectUploadInitResDTO{FileId: file.FileId, Status: multipartStatusCompleted}, nil
	}

	lockToken, locked, err := a.fileCache.AcquireFileInitLock(ctx, dto.UploaderId, fileHash, a.multipartInitLockTTL())
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, ErrUploadBusy
	}
	defer func() {
		_ = a.fileCache.ReleaseFileInitLock(context.Background(), dto.UploaderId, fileHash, lockToken)
	}()

	if file, err := a.findFileByUploaderAndHash(ctx, dto.UploaderId, fileHash); err != nil {
		return nil, err
	} else if file != nil {
		return &DirectUploadInitResDTO{FileId: file.FileId, Status: multipartStatusCompleted}, nil
	}

	now := time.Now()
	existing, err := a.fileUploadRepository.FindUploadingByUploaderAndHash(ctx, dto.UploaderId, fileHash, now.UnixMilli())
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.UploadMode != directUploadMode {
			return nil, ErrUploadBusy
		}
		return a.presignDirectUpload(ctx, existing)
	}

	fileId, err := a.idGenerator.Generate()
	if err != nil {
		return nil, err
	}
	uploadId, err := a.idGenerator.Generate()
	if err != nil {
		return nil, err
	}
	ttl := a.directUploadURLTTL()
	record := filerepo.FileUploadRecord{
		UploadId:     uploadId,
		FileId:       fileId,
		UploaderId:   dto.UploaderId,
		UploadMode:   directUploadMode,
		FileHash:     fileHash,
		ObjectKey:    a.buildObjectKey(dto.UploaderId, fileId, dto.FileName),
		FileName:     dto.FileName,
		ContentType:  dto.ContentType,
		ExpectedSize: dto.Size,
		Status:       multipartStatusUploading,
		ExpiresAt:    now.Add(ttl).UnixMilli(),
		CreatedAt:    now.UnixMilli(),
		UpdatedAt:    now.UnixMilli(),
	}
	if err := a.fileUploadRepository.Create(ctx, record); err != nil {
		return nil, err
	}

	result, err := a.presignDirectUpload(ctx, &record)
	if err != nil {
		_ = a.fileUploadRepository.Delete(context.Background(), uploadId)
		return nil, err
	}
	if err := a.fileCache.SetActiveUpload(ctx, dto.UploaderId, fileHash, uploadId, ttl); err != nil {
		log.Printf("写入直传活跃上传缓存失败：上传=%s 错误=%v", uploadId, err)
	}
	return result, nil
}

func (a *FileApplication) presignDirectUpload(ctx context.Context, record *filerepo.FileUploadRecord) (*DirectUploadInitResDTO, error) {
	ttl := time.Until(time.UnixMilli(record.ExpiresAt))
	if ttl <= 0 {
		return nil, ErrInvalidUpload
	}
	url, err := a.storage.PresignedPutURL(ctx, record.ObjectKey, ttl)
	if err != nil {
		return nil, err
	}
	return &DirectUploadInitResDTO{
		UploadId:  record.UploadId,
		FileId:    record.FileId,
		Status:    multipartStatusUploading,
		URL:       url,
		ExpiresAt: time.UnixMilli(record.ExpiresAt).Unix(),
	}, nil
}

func (a *FileApplication) CompleteDirectUpload(ctx context.Context, uploadId, uploaderId string) (*FileDTO, error) {
	if uploadId == "" || uploaderId == "" {
		return nil, ErrInvalidUpload
	}
	record, err := a.fileUploadRepository.GetByID(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	if record == nil || record.UploadMode != directUploadMode {
		return nil, ErrInvalidUpload
	}
	if record.UploaderId != uploaderId {
		return nil, ErrUploadUnauthorized
	}
	if record.Status == multipartStatusCompleted {
		return a.GetFileForUser(ctx, record.FileId, uploaderId)
	}
	if record.Status != multipartStatusUploading || record.ExpiresAt <= time.Now().UnixMilli() {
		return nil, ErrInvalidUpload
	}

	unlock, lockLost, err := a.acquireFileCompleteLock(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	defer unlock()

	record, err = a.fileUploadRepository.GetByID(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	if record == nil || record.UploaderId != uploaderId || record.UploadMode != directUploadMode {
		return nil, ErrInvalidUpload
	}
	if record.Status == multipartStatusCompleted {
		return a.GetFileForUser(ctx, record.FileId, uploaderId)
	}
	if record.Status != multipartStatusUploading || record.ExpiresAt <= time.Now().UnixMilli() {
		return nil, ErrInvalidUpload
	}

	info, err := a.storage.StatObject(ctx, record.ObjectKey)
	if err != nil {
		return nil, ErrUploadNotCompleted
	}
	if info == nil || info.Size != record.ExpectedSize {
		return nil, ErrFileSizeMismatch
	}
	object, err := a.storage.OpenObject(ctx, record.ObjectKey)
	if err != nil {
		return nil, err
	}
	hasher := sha256.New()
	written, hashErr := io.Copy(hasher, object)
	closeErr := object.Close()
	if hashErr != nil {
		return nil, hashErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if written != record.ExpectedSize {
		return nil, ErrFileSizeMismatch
	}
	// 校验hash 是否一致
	if !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), record.FileHash) {
		return nil, ErrFileHashMismatch
	}

	// 判断锁是否续期成功
	select {
	case <-lockLost:
		return nil, ErrMultipartLockLost
	default:
	}

	file := fileentity.NewFile(
		record.FileId,
		record.UploaderId,
		a.storage.Bucket(),
		record.ObjectKey,
		record.FileName,
		record.ContentType,
		record.ExpectedSize,
	)
	file.FileHash = record.FileHash

	err = a.txManager.WithinTransaction(ctx, func(tx any) error {
		fileRepo := a.fileRepository.WithTx(tx)
		fileUploadRepo := a.fileUploadRepository.WithTx(tx)
		if saveErr := fileRepo.Save(ctx, file); saveErr != nil {
			existing, findErr := fileRepo.FindByUploaderAndHash(ctx, uploaderId, record.FileHash)
			if findErr != nil || existing == nil {
				return saveErr
			}
			file = existing
		}
		marked, markErr := fileUploadRepo.MarkCompleted(ctx, uploadId, file.FileId, time.Now().UnixMilli())
		if markErr != nil {
			return markErr
		}
		if !marked {
			return ErrInvalidUpload
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 当出现并发竞态时，可能会出现某个请求出现唯一键索引，因此需要删除此次上传的文件
	if file.ObjectKey != record.ObjectKey {
		a.cleanupObject(record.ObjectKey)
	}
	if err := a.fileCache.Set(ctx, file, a.cacheTTL()); err != nil {
		log.Printf("完成直传后写入文件缓存失败：文件=%s 错误=%v", file.FileId, err)
	}
	if err := a.fileCache.SetFileIDByUploaderAndHash(ctx, uploaderId, record.FileHash, file.FileId, a.cacheTTL()); err != nil {
		log.Printf("完成直传后写入文件哈希缓存失败：文件=%s 错误=%v", file.FileId, err)
	}
	if err := a.fileCache.DeleteActiveUploadIfMatches(ctx, uploaderId, record.FileHash, uploadId); err != nil {
		log.Printf("完成直传后删除活跃上传缓存失败：上传=%s 错误=%v", uploadId, err)
	}
	return toDTO(file), nil
}

func (a *FileApplication) cleanupObject(objectKey string) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.storage.DeleteObject(cleanupCtx, objectKey); err != nil {
		log.Printf("清理直传孤儿文件失败：对象=%s 错误=%v", objectKey, err)
	}
}

func (a *FileApplication) findFileByUploaderAndHash(ctx context.Context, uploaderId, fileHash string) (*fileentity.File, error) {
	if uploaderId == "" || fileHash == "" {
		return nil, nil
	}

	fileId, err := a.fileCache.GetFileIDByUploaderAndHash(ctx, uploaderId, fileHash)
	if err == nil && fileId != "" {
		file, cacheErr := a.fileCache.Get(ctx, fileId)
		if cacheErr == nil && file != nil && file.UploaderId == uploaderId && (file.Status == "" || file.Status == fileentity.FileStatusUploaded) {
			return file, nil
		}
		if cacheErr != nil {
			log.Printf("读取直传文件元数据缓存失败，继续按哈希回源：文件=%s 错误=%v", fileId, cacheErr)
		}
	} else if err != nil {
		log.Printf("读取直传文件哈希缓存失败，回源数据库：用户=%s 哈希=%s 错误=%v", uploaderId, fileHash, err)
	}

	file, err := a.fileRepository.FindByUploaderAndHash(ctx, uploaderId, fileHash)
	if err != nil || file == nil {
		return file, err
	}
	if file.Status != "" && file.Status != fileentity.FileStatusUploaded {
		return nil, nil
	}

	if cacheErr := a.fileCache.Set(ctx, file, a.cacheTTL()); cacheErr != nil {
		log.Printf("回填直传文件缓存失败：文件=%s 错误=%v", file.FileId, cacheErr)
	}
	if cacheErr := a.fileCache.SetFileIDByUploaderAndHash(ctx, uploaderId, fileHash, file.FileId, a.cacheTTL()); cacheErr != nil {
		log.Printf("回填直传文件哈希缓存失败：文件=%s 错误=%v", file.FileId, cacheErr)
	}
	return file, nil
}

func (a *FileApplication) findActiveMultipartUpload(ctx context.Context, uploaderId, fileHash string) (*MultipartInitResDTO, bool, error) {
	if uploadId, err := a.fileCache.GetActiveUploadId(ctx, uploaderId, fileHash); err != nil {
		return nil, false, err
	} else if uploadId != "" {
		meta, err := a.fileCache.GetMultipartUploadMeta(ctx, uploadId)
		if err != nil {
			return nil, false, err
		}
		if meta != nil {
			uploadedParts, err := a.uploadedPartNumbers(ctx, meta.ObjectKey, meta.StorageUploadId)
			if err != nil {
				return nil, false, err
			}
			return &MultipartInitResDTO{
				UploadId: uploadId, FileId: meta.FileId,
				Status: multipartStatusUploading, UploadedParts: uploadedParts,
			}, true, nil
		}
	}

	now := time.Now().UnixMilli()
	record, err := a.fileUploadRepository.FindUploadingByUploaderAndHash(ctx, uploaderId, fileHash, now)
	if err != nil {
		return nil, false, err
	}
	if record == nil {
		return nil, false, nil
	}
	if record.UploadMode != multipartUploadMode || record.StorageUploadId == "" {
		return nil, false, ErrUploadBusy
	}
	meta := filecache.MultipartUploadMeta{
		UploadId:        record.UploadId,
		StorageUploadId: record.StorageUploadId,
		FileId:          record.FileId,
		UploaderId:      record.UploaderId,
		Bucket:          a.storage.Bucket(),
		ObjectKey:       record.ObjectKey,
		FileName:        record.FileName,
		ContentType:     record.ContentType,
		Size:            record.ExpectedSize,
		FileHash:        record.FileHash,
		ChunkSize:       record.ChunkSize,
		TotalChunks:     record.TotalChunks,
		CreatedAt:       record.CreatedAt,
		Status:          record.Status,
	}
	ttl := time.Until(time.UnixMilli(record.ExpiresAt))
	if ttl <= 0 {
		return nil, false, nil
	}
	if err := a.fileCache.SetMultipartUploadMeta(ctx, meta, ttl); err != nil {
		log.Printf("回填分片上传元数据缓存失败：上传=%s 错误=%v", record.UploadId, err)
	}
	if err := a.fileCache.SetActiveUpload(ctx, uploaderId, fileHash, record.UploadId, ttl); err != nil {
		log.Printf("回填活跃上传缓存失败：上传=%s 错误=%v", record.UploadId, err)
	}
	uploadedParts, err := a.uploadedPartNumbers(ctx, record.ObjectKey, record.StorageUploadId)
	if err != nil {
		return nil, false, err
	}
	return &MultipartInitResDTO{
		UploadId: record.UploadId, FileId: record.FileId,
		Status: multipartStatusUploading, UploadedParts: uploadedParts,
	}, true, nil
}

func (a *FileApplication) InitMultipartUpload(ctx context.Context, dto MultipartInitDTO) (*MultipartInitResDTO, error) {
	fileHash := normalizeFileHash(dto.FileHash)
	if dto.UploaderId == "" || dto.FileName == "" || dto.Size <= 0 ||
		fileHash == "" || dto.ChunkSize <= 0 || dto.TotalChunks <= 0 {
		return nil, ErrInvalidPart
	}
	if a.options.MaxFileSize > 0 && dto.Size > a.options.MaxFileSize {
		return nil, ErrFileTooLarge
	}
	expectedChunks := dto.Size / dto.ChunkSize
	if dto.Size%dto.ChunkSize != 0 {
		expectedChunks++
	}
	if int64(dto.TotalChunks) != expectedChunks {
		return nil, ErrInvalidPart
	}
	if a.options.MaxMultipartParts > 0 && dto.TotalChunks > a.options.MaxMultipartParts {
		return nil, ErrTooManyParts
	}

	if file, err := a.findFileByUploaderAndHash(ctx, dto.UploaderId, fileHash); err != nil {
		return nil, err
	} else if file != nil {
		return &MultipartInitResDTO{FileId: file.FileId, Status: multipartStatusCompleted}, nil
	}

	if result, found, err := a.findActiveMultipartUpload(ctx, dto.UploaderId, fileHash); err != nil || found {
		return result, err
	}

	// 新上传文件
	lockToken, locked, err := a.fileCache.AcquireFileInitLock(ctx, dto.UploaderId, fileHash, a.multipartInitLockTTL())
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, ErrUploadBusy
	}
	defer func() {
		_ = a.fileCache.ReleaseFileInitLock(context.Background(), dto.UploaderId, fileHash, lockToken)
	}()

	// 二次检查，避免出现窗口竞态问题
	if file, err := a.findFileByUploaderAndHash(ctx, dto.UploaderId, fileHash); err != nil {
		return nil, err
	} else if file != nil {
		return &MultipartInitResDTO{FileId: file.FileId, Status: multipartStatusCompleted}, nil
	}
	if result, found, err := a.findActiveMultipartUpload(ctx, dto.UploaderId, fileHash); err != nil || found {
		return result, err
	}

	// uploadId 依旧为空，确定是新文件上传
	fileId, err := a.idGenerator.Generate()
	if err != nil {
		return nil, err
	}

	uploadId, err := a.idGenerator.Generate()
	if err != nil {
		return nil, err
	}

	objectKey := a.buildObjectKey(dto.UploaderId, fileId, dto.FileName)
	storageUploadId, err := a.storage.CreateMultipartUpload(
		ctx,
		objectKey,
		dto.ContentType,
	)
	if err != nil {
		return nil, err
	}

	meta := filecache.MultipartUploadMeta{
		UploadId:        uploadId,
		StorageUploadId: storageUploadId,
		FileId:          fileId,
		UploaderId:      dto.UploaderId,
		Bucket:          a.storage.Bucket(),
		ObjectKey:       objectKey,
		FileName:        dto.FileName,
		ContentType:     dto.ContentType,
		Size:            dto.Size,
		FileHash:        fileHash,
		ChunkSize:       dto.ChunkSize,
		TotalChunks:     dto.TotalChunks,
		CreatedAt:       time.Now().UnixMilli(),
		Status:          multipartStatusUploading,
	}
	now := time.Now().UnixMilli()
	if err := a.fileUploadRepository.Create(ctx, filerepo.FileUploadRecord{
		UploadId:        meta.UploadId,
		FileId:          meta.FileId,
		UploaderId:      meta.UploaderId,
		UploadMode:      multipartUploadMode,
		StorageUploadId: meta.StorageUploadId,
		FileHash:        meta.FileHash,
		ObjectKey:       meta.ObjectKey,
		FileName:        meta.FileName,
		ContentType:     meta.ContentType,
		ExpectedSize:    meta.Size,
		ChunkSize:       meta.ChunkSize,
		TotalChunks:     meta.TotalChunks,
		Status:          multipartStatusUploading,
		ExpiresAt:       now + a.multipartTTL().Milliseconds(),
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		_ = a.storage.AbortMultipartUpload(ctx, meta.ObjectKey, storageUploadId)
		return nil, err
	}

	if err := a.fileCache.SetMultipartUploadMeta(ctx, meta, a.multipartTTL()); err != nil {
		// 取消 minio 上传
		_ = a.storage.AbortMultipartUpload(ctx, meta.ObjectKey, storageUploadId)
		_ = a.fileUploadRepository.Delete(ctx, uploadId)
		return nil, err
	}

	if err := a.fileCache.SetActiveUpload(ctx, dto.UploaderId, fileHash, uploadId, a.multipartTTL()); err != nil {
		_ = a.storage.AbortMultipartUpload(ctx, meta.ObjectKey, storageUploadId)
		_ = a.fileCache.DeleteMultipartUploadMeta(ctx, uploadId)
		_ = a.fileUploadRepository.Delete(ctx, uploadId)
		return nil, err
	}

	return &MultipartInitResDTO{
		UploadId:      uploadId,
		FileId:        fileId,
		Status:        multipartStatusUploading,
		UploadedParts: []int{},
	}, nil
}

func (a *FileApplication) PresignMultipartParts(ctx context.Context, uploadId string, uploaderId string, partNumbers []int) ([]MultipartPartURLDTO, error) {
	meta, err := a.fileCache.GetMultipartUploadMeta(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, ErrInvalidUpload
	}
	if meta.UploaderId != uploaderId {
		return nil, ErrUploadUnauthorized
	}
	if meta.Status != multipartStatusUploading {
		return nil, ErrInvalidUpload
	}
	seen := make(map[int]struct{}, len(partNumbers))

	// 后端返回切片上传的 url，由前端通过 Promise.all 实现并发上传给 minio
	result := make([]MultipartPartURLDTO, 0, len(partNumbers))
	for _, partNumber := range partNumbers {
		if partNumber <= 0 || partNumber > meta.TotalChunks {
			return nil, ErrInvalidPart
		}
		if _, ok := seen[partNumber]; ok {
			continue
		}
		seen[partNumber] = struct{}{}
		url, err := a.storage.PresignMultipartPart(ctx, meta.ObjectKey, meta.StorageUploadId, partNumber, a.partURLTTL())
		if err != nil {
			return nil, err
		}
		result = append(result, MultipartPartURLDTO{UploadId: uploadId, PartNumber: partNumber, URL: url})
	}
	return result, nil
}

func (a *FileApplication) acquireFileCompleteLock(ctx context.Context, uploadId string) (func(), <-chan struct{}, error) {
	lockTTL := a.multipartCompleteLockTTL()
	lockToken, locked, err := a.fileCache.AcquireFileCompleteLock(ctx, uploadId, lockTTL)
	if err != nil {
		return nil, nil, err
	}

	if !locked {
		return nil, nil, ErrUploadBusy
	}

	reneCtx, stopRene := context.WithCancel(context.Background())
	reneInterval := lockTTL / 3
	if reneInterval <= 0 {
		reneInterval = time.Second
	}

	// 交给业务层判断锁是否还在
	lockLost := make(chan struct{})
	renewDone := make(chan struct{})

	go func() {
		defer close(renewDone)
		ticker := time.NewTicker(reneInterval)
		defer ticker.Stop()

		for {
			select {
			case <-reneCtx.Done():
				return
			case <-ticker.C:
				renewCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				renewed, err := a.fileCache.RenewFileCompleteLock(renewCtx, uploadId, lockToken, lockTTL)
				cancel()
				if err != nil || !renewed {
					if err == nil {
						err = ErrMultipartLockLost
					}
					log.Printf("续期分片合并锁失败：上传=%s 错误=%v", uploadId, err)
					close(lockLost)
					return
				}
			}
		}
	}()

	return func() {
		stopRene()
		// 当业务层结束合并时，需要等待协程成功释放资源
		<-renewDone
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = a.fileCache.ReleaseFileCompleteLock(unlockCtx, uploadId, lockToken)
	}, lockLost, nil
}

func (a *FileApplication) loadMultipartMeta(ctx context.Context, uploadId string, uploaderId string) (*filecache.MultipartUploadMeta, error) {
	meta, err := a.fileCache.GetMultipartUploadMeta(ctx, uploadId)

	if err != nil {
		return nil, err
	}

	if meta == nil {
		return nil, ErrInvalidUpload
	}

	if meta.UploaderId != uploaderId {
		return nil, ErrUploadUnauthorized
	}

	return meta, nil
}

func (a *FileApplication) tryReturnCompletedFile(ctx context.Context, meta *filecache.MultipartUploadMeta, userId string) (*FileDTO, bool, error) {
	existing, err := a.fileRepository.GetByID(ctx, meta.FileId)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		return nil, false, nil
	}

	if _, err := a.fileUploadRepository.MarkCompleted(ctx, meta.UploadId, meta.FileId, time.Now().UnixMilli()); err != nil {
		return nil, false, err
	}
	meta.Status = multipartStatusCompleted
	if err := a.fileCache.SetMultipartUploadMeta(ctx, *meta, a.multipartTTL()); err != nil {
		log.Printf("修复已完成上传任务缓存失败：上传=%s 错误=%v", meta.UploadId, err)
	}

	file, err := a.GetFileForUser(ctx, meta.FileId, userId)
	if err != nil {
		return nil, false, err
	}
	return file, true, nil
}

func (a *FileApplication) expectedMultipartPartSize(meta *filecache.MultipartUploadMeta, partNumber int) int64 {
	if partNumber == meta.TotalChunks {
		return meta.Size - int64(meta.TotalChunks-1)*meta.ChunkSize
	}

	return meta.ChunkSize
}

func (a *FileApplication) resolveCompleteMultiParts(meta *filecache.MultipartUploadMeta, parts []objectstorage.MultipartPart) ([]objectstorage.MultipartPart, error) {
	partByNumber := make(map[int]objectstorage.MultipartPart)
	invalidParts := make([]int, 0)

	for _, part := range parts {
		if part.PartNumber <= 0 || part.PartNumber > meta.TotalChunks {
			invalidParts = append(invalidParts, part.PartNumber)
			continue
		}

		if part.ETag == "" {
			invalidParts = append(invalidParts, part.PartNumber)
			continue
		}

		if part.Size != a.expectedMultipartPartSize(meta, part.PartNumber) {
			invalidParts = append(invalidParts, part.PartNumber)
			continue
		}

		partByNumber[part.PartNumber] = part
	}

	missingParts := make([]int, 0)
	for i := 1; i <= meta.TotalChunks; i++ {
		if _, ok := partByNumber[i]; !ok {
			missingParts = append(missingParts, i)
		}
	}

	if len(missingParts) > 0 || len(invalidParts) > 0 {
		return nil, &UploadIncompleteError{
			MissingParts: missingParts,
			InvalidParts: invalidParts,
		}
	}

	// 对 multiparts 进行排序
	completeParts := make([]objectstorage.MultipartPart, 0, meta.TotalChunks)
	for partNumber := 1; partNumber <= meta.TotalChunks; partNumber++ {
		part := partByNumber[partNumber]
		completeParts = append(completeParts, objectstorage.MultipartPart{
			PartNumber: partNumber,
			ETag:       part.ETag,
		})
	}

	return completeParts, nil
}

func (a *FileApplication) completeMultipartObject(ctx context.Context, meta *filecache.MultipartUploadMeta, parts []objectstorage.MultipartPart) error {
	return a.storage.CompleteMultipartUpload(ctx, meta.ObjectKey, meta.StorageUploadId, parts)
}

func (a *FileApplication) afterMultipartCompleted(ctx context.Context, meta *filecache.MultipartUploadMeta, entity *fileentity.File) {
	if err := a.fileCache.Set(ctx, entity, a.cacheTTL()); err != nil {
		log.Printf("完成分片上传后写入文件缓存失败：文件=%s 错误=%v", entity.FileId, err)
	}

	if err := a.fileCache.SetFileIDByUploaderAndHash(ctx, meta.UploaderId, meta.FileHash, meta.FileId, a.multipartTTL()); err != nil {
		log.Printf("完成分片上传后写入文件哈希缓存失败：文件=%s 错误=%v", meta.FileId, err)
	}

	meta.Status = multipartStatusCompleted
	if err := a.fileCache.SetMultipartUploadMeta(ctx, *meta, a.multipartTTL()); err != nil {
		log.Printf("完成分片上传后更新上传任务缓存失败：上传=%s 错误=%v", meta.UploadId, err)
	}

	if err := a.fileCache.DeleteActiveUploadIfMatches(ctx, meta.UploaderId, meta.FileHash, meta.UploadId); err != nil {
		log.Printf("完成分片上传后删除活跃上传缓存失败：上传=%s 错误=%v", meta.UploadId, err)
	}
}

func (a *FileApplication) CompleteMultipartUpload(ctx context.Context, uploadId string, uploaderId string) (*FileDTO, error) {
	meta, err := a.loadMultipartMeta(ctx, uploadId, uploaderId)
	if err != nil {
		return nil, err
	}

	if uploaderId != meta.UploaderId {
		return nil, ErrUploadUnauthorized
	}

	if meta.Status == multipartStatusCompleted {
		return a.GetFileForUser(ctx, meta.FileId, uploaderId)
	}
	if meta.Status != multipartStatusUploading {
		return nil, ErrInvalidUpload
	}

	// 加锁开始上传
	unlock, lockLost, err := a.acquireFileCompleteLock(ctx, uploadId)
	if err != nil {
		return nil, err
	}
	defer unlock()

	// 二次检查
	meta, err = a.loadMultipartMeta(ctx, uploadId, uploaderId)
	if err != nil {
		return nil, err
	}
	if meta.Status == multipartStatusCompleted {
		return a.GetFileForUser(ctx, meta.FileId, uploaderId)
	}

	// 判断是否是缓存未及时更新
	if file, ok, err := a.tryReturnCompletedFile(ctx, meta, uploaderId); err != nil || ok {
		return file, err
	}

	parts, err := a.storage.ListMultipartParts(ctx, meta.ObjectKey, meta.StorageUploadId)
	if err != nil {
		return nil, err
	}

	completePart, err := a.resolveCompleteMultiParts(meta, parts)
	if err != nil {
		return nil, err
	}

	entity := fileentity.NewFile(
		meta.FileId,
		meta.UploaderId,
		meta.Bucket,
		meta.ObjectKey,
		meta.FileName,
		meta.ContentType,
		meta.Size,
	)
	entity.FileHash = meta.FileHash

	select {
	case <-lockLost:
		return nil, ErrMultipartLockLost
	default:
	}

	if err := a.completeMultipartObject(ctx, meta, completePart); err != nil {
		return nil, err
	}

	select {
	case <-lockLost:
		// 合并可能已经完成，不能删除对象，交给孤儿文件清理任务处理。
		return nil, ErrMultipartLockLost
	default:
	}

	var file *fileentity.File
	err = a.txManager.WithinTransaction(ctx, func(tx any) error {
		if saveErr := a.fileRepository.WithTx(tx).Save(ctx, entity); saveErr != nil {
			// 同一 uploadId 可能已经被其他请求写入，先按 fileId 判断，避免误删已完成对象。
			if existing, findErr := a.fileRepository.WithTx(tx).GetByID(ctx, entity.FileId); findErr == nil && existing != nil {
				if _, markErr := a.fileUploadRepository.WithTx(tx).MarkCompleted(ctx, meta.UploadId, existing.FileId, time.Now().UnixMilli()); markErr != nil {
					return markErr
				}

				file = existing
				return nil
			}

			return saveErr
		}

		marked, err := a.fileUploadRepository.WithTx(tx).MarkCompleted(ctx, meta.UploadId, entity.FileId, time.Now().UnixMilli())
		if err != nil {
			return err
		}
		if !marked {
			return ErrInvalidUpload
		}

		return nil
	})

	if err != nil {
		reconcileCtx, reconcileCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		existing, findErr := a.fileRepository.GetByID(reconcileCtx, entity.FileId)
		reconcileCancel()
		if findErr != nil {
			log.Printf("合并后数据库状态回查失败，跳过对象删除：文件=%s 错误=%v", entity.FileId, findErr)
			return nil, err
		}
		if existing != nil {
			return toDTO(existing), nil
		}

		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		if cleanupErr := a.storage.DeleteObject(cleanupCtx, entity.ObjectKey); cleanupErr != nil {
			log.Printf("清理分片合并孤儿文件失败：对象=%s 错误=%v", entity.ObjectKey, cleanupErr)
		}
		cancel()
		return nil, err
	}

	a.afterMultipartCompleted(ctx, meta, entity)

	if file != nil {
		return toDTO(file), nil
	}

	return toDTO(entity), nil
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

func (a *FileApplication) GetAttachmentAccessURL(ctx context.Context, userId string, attachmentId string) (*AttachmentAccessURLDTO, error) {
	if userId == "" || attachmentId == "" {
		return nil, ErrUploadUnauthorized
	}
	if a.messageAttactmentRepository == nil {
		return nil, ErrUploadUnauthorized
	}
	attachment, err := a.messageAttactmentRepository.FindUserAccessAttachment(ctx, userId, attachmentId)
	if err != nil {
		return nil, err
	}
	if attachment == nil {
		return nil, ErrFileNotFound
	}

	var (
		file *fileentity.File
		ok   bool
	)

	file, err = a.fileCache.Get(ctx, attachment.FileId)
	if err != nil {
		log.Printf("读取文件缓存失败，继续回源：文件=%s 错误=%v", attachment.FileId, err)
		file = nil
	}
	if file == nil {
		sfKey := "file-meta:" + attachment.FileId
		ch := a.sf.DoChan(sfKey, func() (any, error) {
			loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancel()

			cached, err := a.fileCache.Get(loadCtx, attachment.FileId)
			if err != nil {
				log.Printf("singleflight 内读取文件缓存失败，继续回源：文件=%s 错误=%v", attachment.FileId, err)
			}
			if cached != nil {
				return cached, nil
			}

			loaded, err := a.fileRepository.GetByID(loadCtx, attachment.FileId)
			if err != nil {
				return nil, err
			}

			if loaded == nil {
				return nil, ErrFileNotFound
			}

			if err := a.fileCache.Set(
				loadCtx,
				loaded,
				a.cacheTTL(),
			); err != nil {
				log.Printf("回填文件缓存失败：文件=%s 错误=%v", loaded.FileId, err)
			}
			return loaded, nil
		})

		select {
		case result := <-ch:
			if result.Err != nil {
				return nil, result.Err
			}

			file, ok = result.Val.(*fileentity.File)
			if !ok || file == nil {
				return nil, ErrFileNotFound
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if file.Status != "" && file.Status != fileentity.FileStatusUploaded {
		return nil, ErrFileNotFound
	}

	ttl := a.urlTTL()
	url, err := a.storage.PresignedGetURL(ctx, file.ObjectKey, ttl)
	if err != nil {
		return nil, err
	}
	return &AttachmentAccessURLDTO{
		AttachmentId: attachment.AttachmentId,
		FileName:     file.FileName,
		ContentType:  file.ContentType,
		Size:         file.Size,
		URL:          url,
		ExpiresAt:    time.Now().Add(ttl).Unix(),
	}, nil
}

func (a *FileApplication) GetFileForUser(ctx context.Context, fileId, userId string) (*FileDTO, error) {
	var (
		file *fileentity.File
		err  error
	)

	file, err = a.fileCache.Get(ctx, fileId)
	if err != nil {
		return nil, err
	}

	if file == nil {
		file, err = a.fileRepository.GetByID(ctx, fileId)
		if err != nil {
			return nil, err
		}
		if file == nil {
			return nil, ErrFileNotFound
		}
	}
	if file.Status != "" && file.Status != fileentity.FileStatusUploaded {
		return nil, fmt.Errorf("文件当前不可引用")
	}

	if file.UploaderId != userId {
		return nil, ErrUploadUnauthorized
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

// 合并文件分片上传的时间，这里需要自动续期，否则若 MinIO 合并超过 TTL，锁自动失效，可能出现并发合并
func (a *FileApplication) multipartCompleteLockTTL() time.Duration {
	if a.options.MultipartCompleteLockTTL <= 0 {
		return 30 * time.Second
	}
	return a.options.MultipartCompleteLockTTL
}

func (a *FileApplication) directUploadLockTTL() time.Duration {
	if a.options.DirectUploadLockTTL <= 0 {
		return 10 * time.Minute
	}
	return a.options.DirectUploadLockTTL
}

func (a *FileApplication) directUploadURLTTL() time.Duration {
	if a.options.DirectUploadURLTTL <= 0 {
		return 30 * time.Minute
	}
	return a.options.DirectUploadURLTTL
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
