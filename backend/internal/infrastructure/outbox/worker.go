package outbox

import (
	"IM_backend/configs"
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
	txManager  txmanager.TxManager
	outboxRepo outboxport.Repository
	publisher  eventbus.Publisher
	options    configs.OutboxConfig
	pool       *OutboxPool
}

func NewWorker(
	txManager txmanager.TxManager,
	outboxRepo outboxport.Repository,
	publisher eventbus.Publisher,
	options configs.OutboxConfig,
) *OutboxWorker {
	return &OutboxWorker{
		txManager:  txManager,
		outboxRepo: outboxRepo,
		publisher:  publisher,
		options:    options,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	w.pool = NewOutboxPool(w.options.WorkerCount, w.options.QueueSize, func(ctx context.Context, item *outboxport.Entry) error {
		return w.dispatchOne(ctx, item)
	})
	w.pool.Start(ctx)
	log.Printf("消息出箱池已启动：workers=%d queueSize=%d", w.options.WorkerCount, w.options.QueueSize)
	defer w.pool.Wait()

	ticker := time.NewTicker(time.Duration(w.options.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()

	drain := func() {
		for ctx.Err() == nil {
			processed, err := w.dispatchPendingOnce(ctx)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("消息出箱任务执行失败：%v", err)
				}
				return
			}
			if !processed {
				return
			}
		}
	}

	// 启动后立即 drain；积压时连续领取，不等待下一次 ticker。
	drain()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			drain()
		}
	}
}

func (w *OutboxWorker) dispatchPendingOnce(ctx context.Context) (bool, error) {
	if w.txManager == nil || w.outboxRepo == nil || w.publisher == nil {
		return false, errors.New("消息出箱工作器依赖未配置")
	}

	var batch []*outboxport.Entry
	now := time.Now()
	err := w.txManager.WithinTransaction(ctx, func(tx any) error {
		items, err := w.outboxRepo.WithTx(tx).ClaimPending(
			ctx,
			now,
			now.Add(-time.Duration(w.options.StaleAfterSeconds)*time.Second),
			w.options.BatchSize,
		)
		if err != nil {
			return err
		}
		batch = items
		return nil
	})
	if err != nil {
		return false, err
	}

	for _, item := range batch {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		if err := w.pool.Submit(ctx, item); err != nil {
			return false, err
		}
	}

	return len(batch) > 0, nil
}

func (w *OutboxWorker) dispatchOne(ctx context.Context, item *outboxport.Entry) error {
	if err := w.publisher.Publish(ctx, eventbus.IntegrationEvent{
		EventID:      item.ID,
		Name:         item.EventType,
		PartitionKey: item.MessageKey,
		Payload:      item.Payload,
	}); err != nil {
		return w.markRetry(ctx, item, err.Error())
	}
	return w.outboxRepo.MarkSent(ctx, item.ID, item.LockToken, time.Now())
}

func (w *OutboxWorker) markRetry(ctx context.Context, item *outboxport.Entry, lastError string) error {
	if item.RetryCount >= w.options.MaxRetries {
		return w.outboxRepo.MarkDead(ctx, item.ID, item.LockToken, lastError)
	}

	baseRetryWait := time.Duration(w.options.BaseRetryWaitSeconds) * time.Second
	retryDelay := baseRetryWait * time.Duration(int(math.Pow(2, float64(item.RetryCount))))
	if retryDelay > 5*time.Minute {
		retryDelay = 5 * time.Minute
	}
	if retryDelay <= 0 {
		retryDelay = baseRetryWait
	}

	return w.outboxRepo.MarkRetry(
		ctx,
		item.ID,
		item.LockToken,
		time.Now().Add(retryDelay),
		lastError,
	)
}
