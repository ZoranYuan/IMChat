package outbox

import (
	"context"
	"time"

	idport "IM_backend/internal/application/ports/id"
	outboxport "IM_backend/internal/application/ports/outbox"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db          *gorm.DB
	idGenerator idport.Generator
}

func NewRepository(db *gorm.DB, idGenerator idport.Generator) *Repository {
	return &Repository{db: db, idGenerator: idGenerator}
}

func (r *Repository) WithTx(tx any) outboxport.Repository {
	return &Repository{db: tx.(*gorm.DB), idGenerator: r.idGenerator}
}

func toModel(e *outboxport.Entry) *model.OutboxRecord {
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

	return &model.OutboxRecord{
		ID:          e.ID,
		EventType:   e.EventType,
		Topic:       e.EventType,
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

func toEntry(m *model.OutboxRecord) *outboxport.Entry {
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

	return &outboxport.Entry{
		ID:          m.ID,
		EventType:   m.EventType,
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

func (r *Repository) Create(ctx context.Context, outbox *outboxport.Entry) error {
	if outbox == nil {
		return nil
	}
	if outbox.ID == "" {
		id, err := r.idGenerator.Generate()
		if err != nil {
			return err
		}
		outbox.ID = id
	}
	if outbox.Status == "" {
		outbox.Status = outboxport.StatusPending
	}
	if outbox.NextRetryAt.IsZero() {
		outbox.NextRetryAt = time.Now()
	}
	m := toModel(outbox)
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *Repository) ClaimPending(
	ctx context.Context,
	now time.Time,
	staleBefore time.Time,
	limit int,
) ([]*outboxport.Entry, error) {
	if limit <= 0 {
		return []*outboxport.Entry{}, nil
	}
	var models []*model.OutboxRecord
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where(
			"(status = ? AND next_retry_at <= ?) OR (status = ? AND locked_at IS NOT NULL AND locked_at <= ?)",
			outboxport.StatusPending,
			now,
			outboxport.StatusProcessing,
			staleBefore,
		).
		Order("next_retry_at ASC, created_at ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return []*outboxport.Entry{}, nil
	}
	ids := make([]string, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	lockAt := now
	if err := r.db.WithContext(ctx).
		Model(&model.OutboxRecord{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"status":     outboxport.StatusProcessing,
			"locked_at":  &lockAt,
			"last_error": "",
		}).Error; err != nil {
		return nil, err
	}
	result := make([]*outboxport.Entry, 0, len(models))
	for _, m := range models {
		m.Status = outboxport.StatusProcessing
		m.LockedAt = &lockAt
		m.LastError = ""
		result = append(result, toEntry(m))
	}
	return result, nil
}

func (r *Repository) MarkSent(ctx context.Context, id string, sentAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.OutboxRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":    outboxport.StatusSent,
			"sent_at":   sentAt,
			"locked_at": nil,
		}).Error
}

func (r *Repository) MarkRetry(ctx context.Context, id string, nextRetryAt time.Time, lastError string) error {
	return r.db.WithContext(ctx).
		Model(&model.OutboxRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        outboxport.StatusPending,
			"next_retry_at": nextRetryAt,
			"last_error":    lastError,
			"locked_at":     nil,
			"retry_count":   gorm.Expr("retry_count + 1"),
		}).Error
}

func (r *Repository) MarkDead(ctx context.Context, id string, lastError string) error {
	return r.db.WithContext(ctx).
		Model(&model.OutboxRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     outboxport.StatusDead,
			"last_error": lastError,
			"locked_at":  nil,
		}).Error
}
