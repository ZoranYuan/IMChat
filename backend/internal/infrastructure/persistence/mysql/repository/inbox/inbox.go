package inbox

import (
	inboxport "IM_backend/internal/application/ports/inbox"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrEventInProgress = errors.New("事件正在处理中")

type InboxRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *InboxRepository {
	return &InboxRepository{db: db}
}

func (r *InboxRepository) WithTx(tx any) inboxport.InboxRepository {
	return &InboxRepository{db: tx.(*gorm.DB)}
}

func (r *InboxRepository) TryClaim(ctx context.Context, eventID, eventType string, now, staleBefore time.Time) (bool, error) {
	if eventID == "" {
		return true, nil
	}

	claimed := false
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.InboxRecord{
		EventID:     eventID,
		EventType:   eventType,
		Status:      inboxport.StatusProcessing,
		LockedAt:    &now,
		ProcessedAt: now,
	})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 1 {
		return true, nil
	}

	var record model.InboxRecord
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id = ?", eventID).First(&record).Error; err != nil {
		return false, err
	}
	switch record.Status {
	case inboxport.StatusCompleted, inboxport.StatusDead:
		return false, nil
	case inboxport.StatusProcessing:
		if record.LockedAt != nil && record.LockedAt.After(staleBefore) {
			return false, ErrEventInProgress
		}
	}

	result = r.db.WithContext(ctx).Model(&model.InboxRecord{}).
		Where("event_id = ?", eventID).
		Updates(map[string]any{
			"status":      inboxport.StatusProcessing,
			"locked_at":   &now,
			"retry_count": gorm.Expr("retry_count + 1"),
			"last_error":  "",
		})
	if result.Error != nil {
		return false, result.Error
	}
	claimed = result.RowsAffected == 1
	return claimed, nil
}

func (r *InboxRepository) MarkCompleted(ctx context.Context, eventID string, processedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.InboxRecord{}).
		Where("event_id = ? AND status = ?", eventID, inboxport.StatusProcessing).
		Updates(map[string]any{
			"status":       inboxport.StatusCompleted,
			"locked_at":    nil,
			"processed_at": processedAt,
		}).Error
}

func (r *InboxRepository) MarkDead(ctx context.Context, eventID string, lastError string, processedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.InboxRecord{}).
		Where("event_id = ? AND status = ?", eventID, inboxport.StatusProcessing).
		Updates(map[string]any{
			"status":       inboxport.StatusDead,
			"locked_at":    nil,
			"last_error":   lastError,
			"processed_at": processedAt,
		}).Error
}
