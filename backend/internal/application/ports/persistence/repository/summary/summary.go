package summary

import "context"

type RunRecord struct {
	SummaryRunID    string
	RoomID          string
	UserID          string
	Status          string
	FromSeq         int64
	ToSeq           int64
	LatestRequestID string
	ResponsePayload string
	CreatedAt       int64
	UpdatedAt       int64
	FinishedAt      *int64
}

type Repository interface {
	CreateRun(ctx context.Context, run RunRecord) error
	FindRun(ctx context.Context, summaryRunID string) (*RunRecord, error)
	FindRunForUpdate(ctx context.Context, summaryRunID string) (*RunRecord, error)
	FindActiveRunByScope(ctx context.Context, userID, roomID string) (*RunRecord, error)
	UpdateStatus(ctx context.Context, runID, status string) error
	SaveResponse(ctx context.Context, runID, requestID, status, payload string, finishedAt *int64) error
	WithTx(tx any) Repository
}
