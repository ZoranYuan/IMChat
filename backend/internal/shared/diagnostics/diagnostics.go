package diagnostics

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	diagnosticsSampleOnce  sync.Once
	diagnosticsSampleEvery uint64 = 1
	diagnosticsCounters    sync.Map
)

func Enabled() bool {
	return os.Getenv("IM_DIAGNOSTICS") == "1"
}

func Logf(format string, args ...any) {
	if !Enabled() || !shouldLogSample(format) {
		return
	}
	log.Printf("[IM_DIAG] "+format, args...)
}

func LogfAlways(format string, args ...any) {
	if Enabled() {
		log.Printf("[IM_DIAG] "+format, args...)
	}
}

func shouldLogSample(format string) bool {
	diagnosticsSampleOnce.Do(func() {
		value, err := strconv.ParseUint(strings.TrimSpace(os.Getenv("IM_DIAGNOSTICS_SAMPLE_EVERY")), 10, 64)
		if err == nil && value > 0 {
			diagnosticsSampleEvery = value
		}
	})
	if diagnosticsSampleEvery <= 1 {
		return true
	}
	counterValue, _ := diagnosticsCounters.LoadOrStore(format, &atomic.Uint64{})
	return counterValue.(*atomic.Uint64).Add(1)%diagnosticsSampleEvery == 0
}

func StartSQLPoolSampler(ctx context.Context, db *sql.DB, interval time.Duration) {
	if !Enabled() || db == nil {
		return
	}
	if interval <= 0 {
		interval = time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		previous := db.Stats()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current := db.Stats()
				LogfAlways("stage=mysql_pool open=%d in_use=%d idle=%d wait_count_delta=%d wait_duration_delta_ms=%d max_idle_closed_delta=%d max_lifetime_closed_delta=%d",
					current.OpenConnections, current.InUse, current.Idle,
					current.WaitCount-previous.WaitCount,
					(current.WaitDuration - previous.WaitDuration).Milliseconds(),
					current.MaxIdleClosed-previous.MaxIdleClosed,
					current.MaxLifetimeClosed-previous.MaxLifetimeClosed)
				previous = current
			}
		}
	}()
}

// StartOutboxBacklogSampler 低频采集待投递和处理中 Outbox 的数量及最老记录年龄。
func StartOutboxBacklogSampler(ctx context.Context, db *sql.DB, interval time.Duration) {
	if !Enabled() || db == nil {
		return
	}
	if interval <= 0 {
		interval = time.Second
	}
	go func() {
		sample := func() {
			var pendingCount, processingCount, pendingOldestAgeUS, processingOldestAgeUS sql.NullInt64
			err := db.QueryRowContext(ctx, `SELECT
				COALESCE(SUM(CASE WHEN status = 'pending' AND next_retry_at <= CURRENT_TIMESTAMP(3) THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END), 0),
				TIMESTAMPDIFF(MICROSECOND, MIN(CASE WHEN status = 'pending' AND next_retry_at <= CURRENT_TIMESTAMP(3) THEN created_at END), CURRENT_TIMESTAMP(3)),
				TIMESTAMPDIFF(MICROSECOND, MIN(CASE WHEN status = 'processing' THEN created_at END), CURRENT_TIMESTAMP(3))
				FROM outboxes
				WHERE status IN ('pending', 'processing')`).Scan(
				&pendingCount, &processingCount, &pendingOldestAgeUS, &processingOldestAgeUS,
			)
			if err != nil {
				LogfAlways("stage=outbox_backlog outcome=error error=%q", err.Error())
				return
			}
			LogfAlways("stage=outbox_backlog pending=%d pending_oldest_age_ms=%d processing=%d processing_oldest_age_ms=%d outcome=ok",
				pendingCount.Int64, pendingOldestAgeUS.Int64/1000,
				processingCount.Int64, processingOldestAgeUS.Int64/1000)
		}
		sample()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sample()
			}
		}
	}()
}

// StartMySQLStatusSampler 低频采集 MySQL 锁等待、Redo 日志等待和文件 I/O 等待计数。
func StartMySQLStatusSampler(ctx context.Context, db *sql.DB, interval time.Duration) {
	if !Enabled() || db == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	go func() {
		previous, err := readMySQLGlobalStatus(ctx, db)
		if err != nil {
			LogfAlways("stage=mysql_status outcome=initial_read_error error=%q", err.Error())
			return
		}
		previousFileWaits, fileWaitErr := readMySQLInnoDBFileWaits(ctx, db)
		if fileWaitErr != nil {
			LogfAlways("stage=mysql_file_wait_probe outcome=unavailable error=%q", fileWaitErr.Error())
		}
		_, lockWaitErr := readMySQLCurrentLockWaits(ctx, db)
		if lockWaitErr != nil {
			LogfAlways("stage=mysql_lock_wait_probe outcome=unavailable error=%q", lockWaitErr.Error())
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, readErr := readMySQLGlobalStatus(ctx, db)
				if readErr != nil {
					LogfAlways("stage=mysql_status outcome=read_error error=%q", readErr.Error())
					continue
				}
				fileWaits, currentFileWaitErr := readMySQLInnoDBFileWaits(ctx, db)
				lockWaits, currentLockWaitErr := readMySQLCurrentLockWaits(ctx, db)
				LogfAlways("stage=mysql_status row_lock_waits_delta=%d row_lock_time_ms_delta=%d current_row_lock_waits=%d innodb_log_waits_delta=%d redo_fsyncs_delta=%d data_fsyncs_delta=%d buffer_pool_wait_free_delta=%d commits_delta=%d threads_running=%d data_lock_waits=%d file_wait_ps_delta=%s outcome=%s",
					statusDelta(current, previous, "Innodb_row_lock_waits"),
					statusDelta(current, previous, "Innodb_row_lock_time"),
					current["Innodb_row_lock_current_waits"],
					statusDelta(current, previous, "Innodb_log_waits"),
					statusDelta(current, previous, "Innodb_os_log_fsyncs"),
					statusDelta(current, previous, "Innodb_data_fsyncs"),
					statusDelta(current, previous, "Innodb_buffer_pool_wait_free"),
					statusDelta(current, previous, "Com_commit"),
					current["Threads_running"], lockWaits,
					formatFileWaitDelta(fileWaits, previousFileWaits),
					mysqlSamplerOutcome(currentFileWaitErr, currentLockWaitErr))
				previous = current
				if currentFileWaitErr == nil {
					previousFileWaits = fileWaits
				}
			}
		}
	}()
}

func readMySQLGlobalStatus(ctx context.Context, db *sql.DB) (map[string]uint64, error) {
	rows, err := db.QueryContext(ctx, `SHOW GLOBAL STATUS WHERE Variable_name IN (
		'Innodb_row_lock_waits', 'Innodb_row_lock_time', 'Innodb_row_lock_current_waits',
		'Innodb_log_waits', 'Innodb_os_log_fsyncs', 'Innodb_data_fsyncs',
		'Innodb_buffer_pool_wait_free', 'Com_commit', 'Threads_running'
	)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make(map[string]uint64)
	for rows.Next() {
		var name, raw string
		if err := rows.Scan(&name, &raw); err != nil {
			return nil, err
		}
		value, err := strconv.ParseUint(raw, 10, 64)
		if err == nil {
			values[name] = value
		}
	}
	return values, rows.Err()
}

type mysqlFileWait struct {
	count  uint64
	waitPS uint64
}

func readMySQLInnoDBFileWaits(ctx context.Context, db *sql.DB) (map[string]mysqlFileWait, error) {
	rows, err := db.QueryContext(ctx, `SELECT EVENT_NAME, COUNT_STAR, SUM_TIMER_WAIT
		FROM performance_schema.events_waits_summary_global_by_event_name
		WHERE EVENT_NAME IN ('wait/io/file/innodb/innodb_log_file', 'wait/io/file/innodb/innodb_data_file')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make(map[string]mysqlFileWait)
	for rows.Next() {
		var name string
		var count, waitPS sql.NullInt64
		if err := rows.Scan(&name, &count, &waitPS); err != nil {
			return nil, err
		}
		values[name] = mysqlFileWait{count: nonNegativeUint(count), waitPS: nonNegativeUint(waitPS)}
	}
	return values, rows.Err()
}

func readMySQLCurrentLockWaits(ctx context.Context, db *sql.DB) (uint64, error) {
	var count uint64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM performance_schema.data_lock_waits").Scan(&count)
	return count, err
}

func statusDelta(current, previous map[string]uint64, name string) uint64 {
	if current[name] < previous[name] {
		return 0
	}
	return current[name] - previous[name]
}

func nonNegativeUint(value sql.NullInt64) uint64 {
	if !value.Valid || value.Int64 < 0 {
		return 0
	}
	return uint64(value.Int64)
}

func formatFileWaitDelta(current, previous map[string]mysqlFileWait) string {
	var parts []string
	for name, value := range current {
		old := previous[name]
		countDelta, waitDelta := uint64(0), uint64(0)
		if value.count >= old.count {
			countDelta = value.count - old.count
		}
		if value.waitPS >= old.waitPS {
			waitDelta = value.waitPS - old.waitPS
		}
		parts = append(parts, strings.TrimPrefix(name, "wait/io/file/innodb/")+":"+strconv.FormatUint(countDelta, 10)+"/"+strconv.FormatUint(waitDelta, 10))
	}
	return strings.Join(parts, ",")
}

func mysqlSamplerOutcome(fileWaitErr, lockWaitErr error) string {
	if fileWaitErr != nil && lockWaitErr != nil {
		return "file_and_lock_wait_unavailable"
	}
	if fileWaitErr != nil {
		return "file_wait_unavailable"
	}
	if lockWaitErr != nil {
		return "lock_wait_unavailable"
	}
	return "ok"
}
