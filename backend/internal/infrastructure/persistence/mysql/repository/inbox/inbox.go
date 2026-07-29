package inbox

import (
	inboxport "IM_backend/internal/application/ports/inbox"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InboxRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *InboxRepository {
	return &InboxRepository{db: db}
}

func (r *InboxRepository) WithTx(tx any) inboxport.InboxRepository {
	return &InboxRepository{db: tx.(*gorm.DB)}
}

func (r *InboxRepository) TryInsert(
	ctx context.Context,
	eventID string,
	eventType string,
) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			DoNothing: true,
		}).
		Create(&model.InboxRecord{
			EventID:     eventID,
			EventType:   eventType,
			ProcessedAt: time.Now(),
		})

	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
}
