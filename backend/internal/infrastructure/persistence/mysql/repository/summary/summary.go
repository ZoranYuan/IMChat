package summary

import (
	summaryrepo "IM_backend/internal/application/ports/persistence/repository/summary"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ summaryrepo.Repository = (*Repository)(nil)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) WithTx(tx any) summaryrepo.Repository { return &Repository{db: tx.(*gorm.DB)} }

func (r *Repository) CreateRun(ctx context.Context, run summaryrepo.RunRecord) error {
	return r.db.WithContext(ctx).Create(toRunModel(run)).Error
}

func (r *Repository) FindRun(ctx context.Context, id string) (*summaryrepo.RunRecord, error) {
	var row model.SummaryRun
	err := r.db.WithContext(ctx).Where("summary_run_id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := toRunRecord(row)
	return &result, nil
}

func (r *Repository) FindRunForUpdate(ctx context.Context, id string) (*summaryrepo.RunRecord, error) {
	var row model.SummaryRun
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("summary_run_id = ?", id).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := toRunRecord(row)
	return &result, nil
}

func (r *Repository) FindActiveRunByScope(ctx context.Context, userID, roomID string) (*summaryrepo.RunRecord, error) {
	var row model.SummaryRun
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND room_id = ? AND status IN ?", userID, roomID, []string{"RUNNING", "WAITING_USER_DECISION"}).
		Order("updated_at DESC, created_at DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := toRunRecord(row)
	return &result, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, runID, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.SummaryRun{}).
		Where("summary_run_id = ?", runID).
		Updates(map[string]any{
			"status":     status,
			"updated_at": gorm.Expr("UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000"),
		}).Error
}

func (r *Repository) SaveResponse(ctx context.Context, runID, requestID, status, payload string, finishedAt *int64) error {
	updates := map[string]any{
		"latest_request_id": requestID,
		"status":            status,
		"response_payload":  payload,
		"updated_at":        gorm.Expr("UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000"),
	}
	if finishedAt != nil {
		updates["finished_at"] = *finishedAt
	}
	return r.db.WithContext(ctx).Model(&model.SummaryRun{}).Where("summary_run_id = ?", runID).Updates(updates).Error
}

func toRunModel(run summaryrepo.RunRecord) *model.SummaryRun {
	return &model.SummaryRun{
		SummaryRunID: run.SummaryRunID, RoomID: run.RoomID, UserID: run.UserID,
		Status: run.Status, FromSeq: run.FromSeq, ToSeq: run.ToSeq,
		LatestRequestID: run.LatestRequestID, ResponsePayload: run.ResponsePayload,
		CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt, FinishedAt: run.FinishedAt,
	}
}

func toRunRecord(row model.SummaryRun) summaryrepo.RunRecord {
	return summaryrepo.RunRecord{
		SummaryRunID: row.SummaryRunID, RoomID: row.RoomID, UserID: row.UserID,
		Status: row.Status, FromSeq: row.FromSeq, ToSeq: row.ToSeq,
		LatestRequestID: row.LatestRequestID, ResponsePayload: row.ResponsePayload,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, FinishedAt: row.FinishedAt,
	}
}
