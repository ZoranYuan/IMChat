package outbox

import (
	eventbus "IM_backend/internal/application/ports/eventbus"
	outboxport "IM_backend/internal/application/ports/outbox"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	"context"
	"errors"
	"log"
	"math"
	"time"
)

type OutboxWorker struct {
	txManager     txmanager.TxManager
	outboxRepo    outboxport.Repository
	publisher     eventbus.Publisher
	batchSize     int
	interval      time.Duration
	staleAfter    time.Duration
	baseRetryWait time.Duration
	maxRetries    int
}

func NewWorker(
	txManager txmanager.TxManager,
	outboxRepo outboxport.Repository,
	publisher eventbus.Publisher,
) *OutboxWorker {
	return &OutboxWorker{
		txManager:     txManager,
		outboxRepo:    outboxRepo,
		publisher:     publisher,
		batchSize:     10,
		interval:      2 * time.Second,
		staleAfter:    30 * time.Second,
		baseRetryWait: 2 * time.Second,
		maxRetries:    10,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		if err := w.dispatchPendingOnce(ctx); err != nil {
			log.Printf("Outbox 工作任务执行失败：%v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *OutboxWorker) dispatchPendingOnce(ctx context.Context) error {
	if w.txManager == nil || w.outboxRepo == nil || w.publisher == nil {
		return errors.New("Outbox Worker 依赖未配置")
	}
	var batch []*outboxport.Entry
	now := time.Now()
	err := w.txManager.WithinTransaction(ctx, func(tx any) error {
		items, err := w.outboxRepo.WithTx(tx).ClaimPending(ctx, now, now.Add(-w.staleAfter), w.batchSize)
		if err != nil {
			return err
		}
		batch = items
		return nil
	})
	if err != nil {
		return err
	}
	for _, item := range batch {
		if item == nil {
			continue
		}
		if err := w.dispatchOne(ctx, item); err != nil {
			log.Printf("分发 Outbox 记录 %s 失败：%v", item.ID, err)
		}
	}
	return nil
}

func (w *OutboxWorker) dispatchOne(ctx context.Context, item *outboxport.Entry) error {
	if err := w.publisher.Publish(ctx, eventbus.IntegrationEvent{
		Name:         item.EventType,
		PartitionKey: item.MessageKey,
		Payload:      item.Payload,
	}); err != nil {
		return w.markRetry(ctx, item, err.Error())
	}
	return w.outboxRepo.MarkSent(ctx, item.ID, time.Now())
}

func (w *OutboxWorker) markRetry(ctx context.Context, item *outboxport.Entry, lastError string) error {
	if item.RetryCount >= w.maxRetries {
		return w.outboxRepo.MarkDead(ctx, item.ID, lastError)
	}
	retryDelay := w.baseRetryWait * time.Duration(int(math.Pow(2, float64(item.RetryCount))))
	if retryDelay > 5*time.Minute {
		retryDelay = 5 * time.Minute
	}
	if retryDelay <= 0 {
		retryDelay = w.baseRetryWait
	}
	return w.outboxRepo.MarkRetry(ctx, item.ID, time.Now().Add(retryDelay), lastError)
}
