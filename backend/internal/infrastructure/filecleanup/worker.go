package filecleanup

import (
	configs "IM_backend/configs"
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	filerepo "IM_backend/internal/application/ports/persistence/repository/file"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	objectstorage "IM_backend/internal/application/ports/storage/object"
	fileentity "IM_backend/internal/domain/file/entity"
	"context"
	"errors"
	"log"
	"math"
	"time"
)

type Worker struct {
	txManager        txmanager.TxManager
	fileUploadRepo   filerepo.FileUploadRepository
	fileRepo         filerepo.FileRepository
	fileCache        filecache.FileCache
	storage          objectstorage.ObjectStorage
	interval         time.Duration
	batchSize        int
	staleAfter       time.Duration
	baseRetryWait    time.Duration
	operationTimeout time.Duration
	maxRetries       int
	orphanAfter      time.Duration
}

func NewWorker(
	txManager txmanager.TxManager,
	fileUploadRepo filerepo.FileUploadRepository,
	fileRepo filerepo.FileRepository,
	fileCache filecache.FileCache,
	storage objectstorage.ObjectStorage,
	config configs.FileCleanupConfig,
) *Worker {
	return &Worker{
		txManager:        txManager,
		fileUploadRepo:   fileUploadRepo,
		fileRepo:         fileRepo,
		fileCache:        fileCache,
		storage:          storage,
		interval:         time.Duration(config.IntervalSeconds) * time.Second,
		batchSize:        config.BatchSize,
		staleAfter:       time.Duration(config.StaleAfterSeconds) * time.Second,
		baseRetryWait:    time.Duration(config.BaseRetryWaitSeconds) * time.Second,
		operationTimeout: time.Duration(config.OperationTimeoutSeconds) * time.Second,
		maxRetries:       config.MaxRetries,
		orphanAfter:      time.Duration(config.OrphanAfterSeconds) * time.Second,
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
	if w.txManager == nil || w.fileUploadRepo == nil || w.fileCache == nil || w.storage == nil {
		return errors.New("文件上传清理任务依赖未配置")
	}

	now := time.Now().UnixMilli()
	staleBefore := now - w.staleAfter.Milliseconds()
	var uploads []filerepo.FileUploadRecord
	if err := w.txManager.WithinTransaction(ctx, func(tx any) error {
		var err error
		uploads, err = w.fileUploadRepo.WithTx(tx).ClaimExpired(ctx, now, staleBefore, w.batchSize)
		return err
	}); err != nil {
		return err
	}

	for _, upload := range uploads {
		if err := w.cleanupOne(ctx, upload); err != nil {
			w.recordFailure(ctx, upload, err)
		}
	}
	if err := w.cleanupOrphanFiles(ctx, now-w.orphanAfter.Milliseconds(), now); err != nil && ctx.Err() == nil {
		log.Printf("未引用文件清理任务执行失败：%v", err)
	}
	return nil
}

func (w *Worker) cleanupOrphanFiles(ctx context.Context, before int64, now int64) error {
	if w.fileRepo == nil {
		return nil
	}
	files, err := w.fileRepo.ListOrphanCandidates(ctx, before, now, w.batchSize)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file == nil {
			continue
		}
		if file.Status == fileentity.FileStatusUploaded {
			marked, err := w.fileRepo.MarkDeleting(ctx, file.FileId, before, now)
			if err != nil || !marked {
				continue
			}
		}
		cleanupCtx, cancel := context.WithTimeout(ctx, w.operationTimeout)
		err = w.storage.DeleteObject(cleanupCtx, file.ObjectKey)
		cancel()
		if err != nil {
			log.Printf("删除未引用文件对象失败：文件=%s 错误=%v", file.FileId, err)
			continue
		}
		deleted, err := w.fileRepo.DeleteDeleting(ctx, file.FileId)
		if err != nil || !deleted {
			log.Printf("删除未引用文件记录失败：文件=%s 错误=%v", file.FileId, err)
			continue
		}
		if err := w.fileCache.Delete(ctx, file.FileId); err != nil {
			log.Printf("删除未引用文件缓存失败：文件=%s 错误=%v", file.FileId, err)
		}
	}
	return nil
}

func (w *Worker) cleanupOne(ctx context.Context, upload filerepo.FileUploadRecord) error {
	cleanupCtx, cancel := context.WithTimeout(ctx, w.operationTimeout)
	defer cancel()

	if upload.StorageUploadId != "" {
		if err := w.storage.AbortMultipartUpload(cleanupCtx, upload.ObjectKey, upload.StorageUploadId); err != nil {
			return err
		}
	} else {
		if err := w.storage.DeleteObject(cleanupCtx, upload.ObjectKey); err != nil {
			return err
		}
	}
	if err := w.fileCache.DeleteMultipartUploadMeta(cleanupCtx, upload.UploadId); err != nil {
		return err
	}
	if err := w.fileCache.DeleteActiveUploadIfMatches(cleanupCtx, upload.UploaderId, upload.FileHash, upload.UploadId); err != nil {
		return err
	}

	marked, err := w.fileUploadRepo.MarkExpired(ctx, upload.UploadId, upload.LockToken, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	if !marked {
		return errors.New("文件上传清理任务锁已失效")
	}
	return nil
}

func (w *Worker) recordFailure(ctx context.Context, upload filerepo.FileUploadRecord, cleanupErr error) {
	now := time.Now()
	lastError := cleanupErr.Error()

	if upload.RetryCount+1 >= w.maxRetries {
		marked, err := w.fileUploadRepo.MarkCleanupFailed(ctx, upload.UploadId, upload.LockToken, lastError, now.UnixMilli())
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
	marked, err := w.fileUploadRepo.MarkCleanupRetry(
		ctx,
		upload.UploadId,
		upload.LockToken,
		now.Add(delay).UnixMilli(),
		lastError,
		now.UnixMilli(),
	)
	if err != nil || !marked {
		log.Printf("记录文件上传清理重试失败：上传=%s 错误=%v 原因=%s", upload.UploadId, err, lastError)
	}
}
