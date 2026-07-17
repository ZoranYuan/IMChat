package mq

import (
	mqport "IM_backend/internal/application/ports/mq"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"gorm.io/gorm"
)

type OutboxWorker struct {
	txManager     txmanager.TxManager
	outboxRepo    messagerepo.MessageOutboxRepository
	taskManager   mqport.TaskManager
	batchSize     int
	interval      time.Duration
	staleAfter    time.Duration
	baseRetryWait time.Duration
}

func NewReadAckOutboxWorker(
	txManager txmanager.TxManager,
	outboxRepo messagerepo.MessageOutboxRepository,
	taskManager mqport.TaskManager,
) *OutboxWorker {
	return &OutboxWorker{
		txManager:     txManager,
		outboxRepo:    outboxRepo,
		taskManager:   taskManager,
		batchSize:     10,
		interval:      2 * time.Second,
		staleAfter:    30 * time.Second,
		baseRetryWait: 2 * time.Second,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		if err := w.dispatchPendingOnce(ctx); err != nil {
			log.Printf("outbox worker failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *OutboxWorker) dispatchPendingOnce(ctx context.Context) error {
	if w.txManager == nil || w.outboxRepo == nil || w.taskManager == nil {
		return nil
	}
	var batch []*messageentity.MessageOutbox
	now := time.Now()
	err := w.txManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
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
			log.Printf("dispatch outbox %s failed: %v", item.ID, err)
		}
	}
	return nil
}

func (w *OutboxWorker) dispatchOne(ctx context.Context, item *messageentity.MessageOutbox) error {
	switch item.EventType {
	case protocol.EventTypeSendMessage:
		return w.dispatchMessage(ctx, item)
	case protocol.EventReadMessageAck:
		return w.dispatchMessageReadAck(ctx, item)
	default:
		return w.markRetry(ctx, item, fmt.Sprintf("unsupported outbox event type: %s", item.EventType))
	}
}

func (w *OutboxWorker) dispatchMessage(ctx context.Context, item *messageentity.MessageOutbox) error {
	var event protocol.MessageEvent
	if err := json.Unmarshal(item.Payload, &event); err != nil {
		return w.markRetry(ctx, item, err.Error())
	}
	if err := w.taskManager.HandleSendMessage(ctx, item.Topic, item.MessageKey, event); err != nil {
		return w.markRetry(ctx, item, err.Error())
	}
	return w.outboxRepo.MarkSent(ctx, item.ID, time.Now())
}

func (w *OutboxWorker) dispatchMessageReadAck(ctx context.Context, item *messageentity.MessageOutbox) error {
	var event protocol.MessageReadAckEvent
	if err := json.Unmarshal(item.Payload, &event); err != nil {
		return w.markRetry(ctx, item, err.Error())
	}
	if err := w.taskManager.HandleReadMessageAck(ctx, item.Topic, item.MessageKey, event); err != nil {
		return w.markRetry(ctx, item, err.Error())
	}
	return w.outboxRepo.MarkSent(ctx, item.ID, time.Now())
}

func (w *OutboxWorker) markRetry(ctx context.Context, item *messageentity.MessageOutbox, lastError string) error {
	retryDelay := w.baseRetryWait * time.Duration(int(math.Pow(2, float64(item.RetryCount))))
	if retryDelay > 5*time.Minute {
		retryDelay = 5 * time.Minute
	}
	if retryDelay <= 0 {
		retryDelay = w.baseRetryWait
	}
	return w.outboxRepo.MarkRetry(ctx, item.ID, time.Now().Add(retryDelay), lastError)
}
