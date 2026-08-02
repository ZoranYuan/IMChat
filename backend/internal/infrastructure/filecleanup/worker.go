package filecleanup

import (
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	objectstorage "IM_backend/internal/application/ports/storage/object"
	"context"
	"errors"
	"log"
	"math"
	"time"
)

const (
	defaultInterval         = time.Minute
	defaultBatchSize        = 50
	defaultStaleAfter       = 5 * time.Minute
	defaultBaseRetryWait    = 30 * time.Second
	defaultOperationTimeout = 30 * time.Second
	defaultMaxRetries       = 10
)

type Options struct {
	Interval         time.Duration
	BatchSize        int
	StaleAfter       time.Duration
	BaseRetryWait    time.Duration
	OperationTimeout time.Duration
	MaxRetries       int
}

type Worker struct {
	txManager        txmanager.TxManager
	uploadRepo       filerepo.MultipartUploadRepository
	fileCache        filecache.FileCache
	storage          objectstorage.ObjectStorage
	interval         time.Duration
	batchSize        int
	staleAfter       time.Duration
	baseRetryWait    time.Duration
	operationTimeout time.Duration
	maxRetries       int
}

func NewWorker(
	txManager txmanager.TxManager,
	uploadRepo filerepo.MultipartUploadRepository,
	fileCache filecache.FileCache,
	storage objectstorage.ObjectStorage,
	options Options,
) *Worker {
	if options.Interval <= 0 {
		options.Interval = defaultInterval
	}
	if options.BatchSize <= 0 {
		options.BatchSize = defaultBatchSize
	}
	if options.StaleAfter <= 0 {
		options.StaleAfter = defaultStaleAfter
	}
	if options.BaseRetryWait <= 0 {
		options.BaseRetryWait = defaultBaseRetryWait
	}
	if options.OperationTimeout <= 0 {
		options.OperationTimeout = defaultOperationTimeout
	}
	if options.MaxRetries <= 0 {
		options.MaxRetries = defaultMaxRetries
	}

	return &Worker{
		txManager:        txManager,
		uploadRepo:       uploadRepo,
		fileCache:        fileCache,
		storage:          storage,
		interval:         options.Interval,
		batchSize:        options.BatchSize,
		staleAfter:       options.StaleAfter,
		baseRetryWait:    options.BaseRetryWait,
		operationTimeout: options.OperationTimeout,
		maxRetries:       options.MaxRetries,
	}
}

func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		if err := w.dispatchPendingOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("文件上传清理任务执行失败：%v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) dispatchPendingOnce(ctx context.Context) error {
	if w.txManager == nil || w.uploadRepo == nil || w.fileCache == nil || w.storage == nil {
		return errors.New("文件上传清理任务依赖未配置")
	}

	now := time.Now().UnixMilli()
	staleBefore := now - w.staleAfter.Milliseconds()
	var uploads []filerepo.MultipartUploadRecord
	if err := w.txManager.WithinTransaction(ctx, func(tx any) error {
		var err error
		uploads, err = w.uploadRepo.WithTx(tx).ClaimExpired(ctx, now, staleBefore, w.batchSize)
		return err
	}); err != nil {
		return err
	}

	for _, upload := range uploads {
		if err := w.cleanupOne(ctx, upload); err != nil {
			w.recordFailure(ctx, upload, err)
		}
	}
	return nil
}

func (w *Worker) cleanupOne(ctx context.Context, upload filerepo.MultipartUploadRecord) error {
	cleanupCtx, cancel := context.WithTimeout(ctx, w.operationTimeout)
	defer cancel()

	if upload.StorageUploadId != "" {
		if err := w.storage.AbortMultipartUpload(cleanupCtx, upload.ObjectKey, upload.StorageUploadId); err != nil {
			return err
		}
	}
	if err := w.fileCache.DeleteMultipartUpload(cleanupCtx, upload.UploadId); err != nil {
		return err
	}
	if err := w.fileCache.DeleteActiveUploadIfMatches(cleanupCtx, upload.UploaderId, upload.FileHash, upload.UploadId); err != nil {
		return err
	}

	lockedAt := int64(0)
	if upload.LockedAt != nil {
		lockedAt = *upload.LockedAt
	}
	marked, err := w.uploadRepo.MarkExpired(ctx, upload.UploadId, lockedAt, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	if !marked {
		return errors.New("文件上传清理任务锁已失效")
	}
	return nil
}

func (w *Worker) recordFailure(ctx context.Context, upload filerepo.MultipartUploadRecord, cleanupErr error) {
	lockedAt := int64(0)
	if upload.LockedAt != nil {
		lockedAt = *upload.LockedAt
	}
	now := time.Now()
	lastError := cleanupErr.Error()

	if upload.RetryCount+1 >= w.maxRetries {
		marked, err := w.uploadRepo.MarkCleanupFailed(ctx, upload.UploadId, lockedAt, lastError, now.UnixMilli())
		if err != nil || !marked {
			log.Printf("标记文件上传清理失败状态失败：上传=%s 错误=%v 原因=%s", upload.UploadId, err, lastError)
			return
		}
		log.Printf("文件上传清理重试耗尽：上传=%s 原因=%s", upload.UploadId, lastError)
		return
	}

	delay := w.baseRetryWait * time.Duration(math.Pow(2, float64(upload.RetryCount)))
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	marked, err := w.uploadRepo.MarkCleanupRetry(
		ctx,
		upload.UploadId,
		lockedAt,
		now.Add(delay).UnixMilli(),
		lastError,
		now.UnixMilli(),
	)
	if err != nil || !marked {
		log.Printf("记录文件上传清理重试失败：上传=%s 错误=%v 原因=%s", upload.UploadId, err, lastError)
	}
}
