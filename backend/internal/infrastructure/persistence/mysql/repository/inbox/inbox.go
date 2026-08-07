package inbox

import (
	inboxport "IM_backend/internal/application/ports/inbox"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
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

func (r *InboxRepository) TryClaim(ctx context.Context, eventID, eventType string, now, staleBefore time.Time) (bool, string, int, error) {
	if eventID == "" {
		return true, "", 1, nil
	}

	claimed := false
	lockToken := uuid.NewString()
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.InboxRecord{
		EventID:     eventID,
		EventType:   eventType,
		Status:      inboxport.StatusProcessing,
		LockedAt:    &now,
		LockToken:   lockToken,
		RetryCount:  1,
		ProcessedAt: now,
	})
	if result.Error != nil {
		return false, "", 0, result.Error
	}
	if result.RowsAffected == 1 {
		return true, lockToken, 1, nil
	}

	// 非第一次
	var record model.InboxRecord
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id = ?", eventID).First(&record).Error; err != nil {
		return false, "", 0, err
	}
	switch record.Status {
	case inboxport.StatusCompleted, inboxport.StatusDead:
		return false, "", record.RetryCount, nil
	case inboxport.StatusProcessing:
		// 未到过期时间
		if record.LockedAt != nil && record.LockedAt.After(staleBefore) {
			return false, "", record.RetryCount, ErrEventInProgress
		}
	}

	result = r.db.WithContext(ctx).Model(&model.InboxRecord{}).
		Where("event_id = ?", eventID).
		Updates(map[string]any{
			"status":      inboxport.StatusProcessing,
			"locked_at":   &now,
			"lock_token":  lockToken,
			"retry_count": gorm.Expr("retry_count + 1"),
			"last_error":  "",
		})
	if result.Error != nil {
		return false, "", 0, result.Error
	}
	claimed = result.RowsAffected == 1
	if !claimed {
		return false, "", record.RetryCount, inboxport.ErrLeaseLost
	}
	return claimed, lockToken, record.RetryCount + 1, nil
}

func (r *InboxRepository) MarkCompleted(ctx context.Context, eventID, lockToken string, processedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.InboxRecord{}).
		Where("event_id = ? AND status = ? AND lock_token = ?", eventID, inboxport.StatusProcessing, lockToken).
		Updates(map[string]any{
			"status":       inboxport.StatusCompleted,
			"locked_at":    nil,
			"lock_token":   "",
			"processed_at": processedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return inboxport.ErrLeaseLost
	}
	return nil
}

func (r *InboxRepository) MarkDead(ctx context.Context, eventID, lockToken, lastError string, processedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.InboxRecord{}).
		Where("event_id = ? AND status = ? AND lock_token = ?", eventID, inboxport.StatusProcessing, lockToken).
		Updates(map[string]any{
			"status":       inboxport.StatusDead,
			"locked_at":    nil,
			"lock_token":   "",
			"last_error":   lastError,
			"processed_at": processedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return inboxport.ErrLeaseLost
	}
	return nil
}
