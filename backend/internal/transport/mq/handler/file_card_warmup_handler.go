package handler

import (
	eventbus "IM_backend/internal/application/ports/eventbus"
	filecache "IM_backend/internal/application/ports/persistence/cache/file"
	objectstorage "IM_backend/internal/application/ports/storage/object"
	fileentity "IM_backend/internal/domain/file/entity"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"time"
)

// FileCardWarmupHandler 只消费消息事务中的文件快照，不再回查 files 表。
// 缓存失效时，附件访问接口仍会回源数据库，因此预热失败不会影响消息可用性。
type FileCardWarmupHandler struct {
	fileCache   filecache.FileCache
	storage     objectstorage.ObjectStorage
	fileCardTTL time.Duration
	urlTTL      time.Duration
}

func NewFileCardWarmupHandler(
	fileCache filecache.FileCache,
	storage objectstorage.ObjectStorage,
	fileCardTTL time.Duration,
	urlTTL time.Duration,
) eventbus.Handler {
	return &FileCardWarmupHandler{
		fileCache:   fileCache,
		storage:     storage,
		fileCardTTL: fileCardTTL,
		urlTTL:      urlTTL,
	}
}

func (h *FileCardWarmupHandler) Handle(ctx context.Context, message eventbus.IncomingEvent) error {
	if h == nil || h.fileCache == nil || h.storage == nil {
		return errors.New("文件卡片预热组件未配置")
	}

	envelope, err := decodeEnvelope(message.Payload)
	if err != nil {
		return eventbus.NonRetryable(err)
	}

	var payload protocol.FileCardWarmupEvent
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return eventbus.NonRetryable(err)
	}
	if payload.AttachmentID == "" || payload.FileID == "" || payload.ObjectKey == "" {
		return eventbus.NonRetryable(errors.New("文件卡片预热事件缺少必要字段"))
	}
	if payload.Status != fileentity.FileStatusUploaded {
		return eventbus.NonRetryable(errors.New("文件当前不可用"))
	}

	cardTTL := h.fileCardTTL
	if cardTTL <= 0 {
		cardTTL = 24 * time.Hour
	}
	if payload.AttachmentExpireAt > 0 {
		remaining := time.Until(time.UnixMilli(payload.AttachmentExpireAt))
		if remaining <= 0 {
			return nil
		}
		if remaining < cardTTL {
			cardTTL = remaining
		}
	}

	card := &filecache.AttachmentFileCard{
		AttachmentID:       payload.AttachmentID,
		FileID:             payload.FileID,
		ObjectKey:          payload.ObjectKey,
		FileName:           payload.FileName,
		ContentType:        payload.ContentType,
		Size:               payload.Size,
		CType:              payload.CType,
		AttachmentExpireAt: payload.AttachmentExpireAt,
	}
	if err := h.fileCache.SetAttachmentFileCardBatch(ctx, []*filecache.AttachmentFileCard{card}, cardTTL); err != nil {
		return err
	}

	urlTTL := h.urlTTL
	if urlTTL <= 0 {
		urlTTL = time.Hour
	}
	if payload.AttachmentExpireAt > 0 {
		remaining := time.Until(time.UnixMilli(payload.AttachmentExpireAt))
		if remaining < urlTTL {
			urlTTL = remaining
		}
	}
	if urlTTL <= 0 {
		return nil
	}

	mediaURL, err := h.storage.PresignedGetURL(
		ctx,
		payload.ObjectKey,
		urlTTL,
		payload.ContentType,
		payload.FileName,
	)
	if err != nil {
		return err
	}
	accessURL := &filecache.AttachmentURL{
		AttachmentID: payload.AttachmentID,
		MediaURL:     mediaURL,
		ExpiresAt:    time.Now().Add(urlTTL).Unix(),
	}
	cacheTTL := urlTTL - 30*time.Second
	if cacheTTL <= 0 {
		cacheTTL = urlTTL
	}
	return h.fileCache.SetAttachmentURLBatch(ctx, []*filecache.AttachmentURL{accessURL}, cacheTTL)
}
