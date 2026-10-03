package inbox

import "errors"

const (
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusDead       = "dead"
)

var ErrLeaseLost = errors.New("inbox lease lost")
var ErrEventInProgress = errors.New("事件正在处理中")
var ErrBatchConflict = errors.New("Inbox 批次包含已有事件")
var ErrEventIDRequired = errors.New("Inbox event_id 不能为空")

type ClaimEvent struct {
	EventID   string
	EventType string
}
