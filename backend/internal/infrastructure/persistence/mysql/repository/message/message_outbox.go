package message

import (
	"context"
	"time"

	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/infrastructure/id/snow"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MessageOutboxRepository struct {
	db        *gorm.DB
	machineID int
}

func NewMessageOutboxRepository(db *gorm.DB, machineID int) *MessageOutboxRepository {
	return &MessageOutboxRepository{db: db, machineID: machineID}
}

func (r *MessageOutboxRepository) WithTx(tx any) messagerepo.MessageOutboxRepository {
	return &MessageOutboxRepository{db: tx.(*gorm.DB), machineID: r.machineID}
}

func toMessageOutboxModel(e *messageentity.MessageOutbox) *model.MessageOutbox {
	if e == nil {
		return nil
	}

	var lockedAt *time.Time
	if e.LockedAt != nil {
		t := *e.LockedAt
		lockedAt = &t
	}

	var sentAt *time.Time
	if e.SentAt != nil {
		t := *e.SentAt
		sentAt = &t
	}

	return &model.MessageOutbox{
		ID:          e.ID,
		EventType:   e.EventType,
		Topic:       e.Topic,
		MessageKey:  e.MessageKey,
		Payload:     e.Payload,
		Status:      e.Status,
		RetryCount:  e.RetryCount,
		NextRetryAt: e.NextRetryAt,
		LockedAt:    lockedAt,
		LastError:   e.LastError,
		SentAt:      sentAt,
	}
}

func toMessageOutboxDomain(m *model.MessageOutbox) *messageentity.MessageOutbox {
	if m == nil {
		return nil
	}

	var lockedAt *time.Time
	if m.LockedAt != nil {
		t := *m.LockedAt
		lockedAt = &t
	}

	var sentAt *time.Time
	if m.SentAt != nil {
		t := *m.SentAt
		sentAt = &t
	}

	return &messageentity.MessageOutbox{
		ID:          m.ID,
		EventType:   m.EventType,
		Topic:       m.Topic,
		MessageKey:  m.MessageKey,
		Payload:     m.Payload,
		Status:      m.Status,
		RetryCount:  m.RetryCount,
		NextRetryAt: m.NextRetryAt,
		LockedAt:    lockedAt,
		LastError:   m.LastError,
		SentAt:      sentAt,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func (r *MessageOutboxRepository) Create(ctx context.Context, outbox *messageentity.MessageOutbox) error {
	if outbox == nil {
		return nil
	}
	if outbox.ID == "" {
		id, err := snow.GenerateSnowID(r.machineID)
		if err != nil {
			return err
		}
		outbox.ID = id
	}
	if outbox.Status == "" {
		outbox.Status = messageentity.MessageOutboxStatusPending
	}
	if outbox.NextRetryAt.IsZero() {
		outbox.NextRetryAt = time.Now()
	}
	m := toMessageOutboxModel(outbox)
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *MessageOutboxRepository) ClaimPending(
	ctx context.Context,
	now time.Time,
	staleBefore time.Time,
	limit int,
) ([]*messageentity.MessageOutbox, error) {
	if limit <= 0 {
		return []*messageentity.MessageOutbox{}, nil
	}
	var models []*model.MessageOutbox
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where(
			"(status = ? AND next_retry_at <= ?) OR (status = ? AND locked_at IS NOT NULL AND locked_at <= ?)",
			messageentity.MessageOutboxStatusPending,
			now,
			messageentity.MessageOutboxStatusProcessing,
			staleBefore,
		).
		Order("next_retry_at ASC, created_at ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return []*messageentity.MessageOutbox{}, nil
	}
	ids := make([]string, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	lockAt := now
	if err := r.db.WithContext(ctx).
		Model(&model.MessageOutbox{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"status":     messageentity.MessageOutboxStatusProcessing,
			"locked_at":  &lockAt,
			"last_error": "",
		}).Error; err != nil {
		return nil, err
	}
	result := make([]*messageentity.MessageOutbox, 0, len(models))
	for _, m := range models {
		m.Status = messageentity.MessageOutboxStatusProcessing
		m.LockedAt = &lockAt
		m.LastError = ""
		result = append(result, toMessageOutboxDomain(m))
	}
	return result, nil
}

func (r *MessageOutboxRepository) MarkSent(ctx context.Context, id string, sentAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.MessageOutbox{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":    messageentity.MessageOutboxStatusSent,
			"sent_at":   sentAt,
			"locked_at": nil,
		}).Error
}

func (r *MessageOutboxRepository) MarkRetry(ctx context.Context, id string, nextRetryAt time.Time, lastError string) error {
	return r.db.WithContext(ctx).
		Model(&model.MessageOutbox{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        messageentity.MessageOutboxStatusPending,
			"next_retry_at": nextRetryAt,
			"last_error":    lastError,
			"locked_at":     nil,
			"retry_count":   gorm.Expr("retry_count + 1"),
		}).Error
}
