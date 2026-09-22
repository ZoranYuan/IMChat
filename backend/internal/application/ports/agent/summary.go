package agent

import (
	"context"
	"time"
)

type SummaryAction string

const (
	SummaryActionStart  SummaryAction = "START"
	SummaryActionRetry  SummaryAction = "RETRY"
	SummaryActionCancel SummaryAction = "CANCEL"
)

type SummaryStatus string

const (
	SummaryStatusRunning             SummaryStatus = "RUNNING" // 后端内部状态，协议响应不返回该值。
	SummaryStatusSucceeded           SummaryStatus = "SUCCEEDED"
	SummaryStatusFailed              SummaryStatus = "FAILED"
	SummaryStatusWaitingUserDecision SummaryStatus = "WAITING_USER_DECISION"
)

type Scope struct {
	RoomID string
	UserID string
}

type RoomMessage struct {
	RoomID    string
	MessageID string
	SenderID  string
	Seq       int64
	Type      int32
	Content   string
	Status    string
	SendTime  time.Time
}

type Citation struct {
	MessageID string `json:"messageId"`
	Seq       int64  `json:"seq"`
}

type SummarySegment struct {
	SummaryContentSegment string     `json:"summaryContentSegment"`
	Citations             []Citation `json:"citations,omitempty"`
}

type SummaryOutput struct {
	SummaryContent []SummarySegment `json:"summaryContent"`
}

type SummaryError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type RetryRequired struct {
	ReasonCode       string   `json:"reasonCode"`
	ValidationIssues []string `json:"validationIssues,omitempty"`
	ReflectionIssues []string `json:"reflectionIssues,omitempty"`
	RepairCount      int32    `json:"repairCount"`
	MaxRepairCount   int32    `json:"maxRepairCount"`
	AllowedActions   []string `json:"allowedActions,omitempty"`
}

type SummaryResponse struct {
	RequestID     string         `json:"requestId"`
	SummaryRunID  string         `json:"summaryRunId"`
	Status        SummaryStatus  `json:"status"`
	Summary       *SummaryOutput `json:"summary,omitempty"`
	Error         *SummaryError  `json:"error,omitempty"`
	RetryRequired *RetryRequired `json:"retryRequired,omitempty"`
}

type SummaryRequest struct {
	RequestID    string
	SummaryRunID string
	Scope        Scope
	Action       SummaryAction
	RoomMessages []RoomMessage
}

type ResumeSummaryRequest struct {
	RequestID    string
	SummaryRunID string
	Scope        Scope
}

type SummaryClient interface {
	RunSummary(ctx context.Context, request SummaryRequest) (*SummaryResponse, error)
	ResumeSummaryState(ctx context.Context, request ResumeSummaryRequest) (*SummaryResponse, error)
}
