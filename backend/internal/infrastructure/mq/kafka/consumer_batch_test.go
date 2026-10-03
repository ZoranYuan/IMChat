package kafka

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	configs "IM_backend/configs"
	eventbus "IM_backend/internal/application/ports/eventbus"
	inboxport "IM_backend/internal/application/ports/inbox"
	persistence "IM_backend/internal/infrastructure/persistence"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	mysqlrepo "IM_backend/internal/infrastructure/persistence/mysql/repository/inbox"
	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type testInboxRow struct {
	status, token string
	locked        time.Time
	attempts      int
}

type testInbox struct {
	mu                                    sync.Mutex
	rows                                  map[string]testInboxRow
	completeErr, renewErr                 error
	claims, completes, renewals, releases int
	singleClaims                          int
}

func (r *testInbox) WithTx(tx any) inboxport.InboxRepository { return tx.(*testInbox) }

func (r *testInbox) WithinTransaction(ctx context.Context, fn func(any) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	tx := &testInbox{rows: make(map[string]testInboxRow), completeErr: r.completeErr}
	for id, row := range r.rows {
		tx.rows[id] = row
	}
	err := fn(tx)
	r.claims += tx.claims
	r.singleClaims += tx.singleClaims
	r.completes += tx.completes
	if err == nil {
		r.rows = tx.rows
	}
	return err
}

func (r *testInbox) TryClaimBatch(ctx context.Context, events []inboxport.ClaimEvent, token string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claims++
	for _, event := range events {
		if _, ok := r.rows[event.EventID]; ok {
			return inboxport.ErrBatchConflict
		}
		// 模拟 INSERT 已写入一部分，验证冲突时整个事务被回滚。
		r.rows[event.EventID] = testInboxRow{status: inboxport.StatusProcessing, token: token, locked: now, attempts: 1}
	}
	return nil
}

func (r *testInbox) CompleteBatch(ctx context.Context, ids []string, token string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completes++
	if r.completeErr != nil {
		return r.completeErr
	}
	for _, id := range ids {
		row := r.rows[id]
		if row.token != token || row.status != inboxport.StatusProcessing {
			return inboxport.ErrLeaseLost
		}
		row.status, row.token = inboxport.StatusCompleted, ""
		r.rows[id] = row
	}
	return nil
}

func (r *testInbox) RenewBatch(ctx context.Context, ids []string, token string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.renewals++
	if r.renewErr != nil {
		return r.renewErr
	}
	for _, id := range ids {
		row := r.rows[id]
		if row.token != token {
			return inboxport.ErrLeaseLost
		}
		row.locked = now
		r.rows[id] = row
	}
	return nil
}

func (r *testInbox) ReleaseBatch(ctx context.Context, ids []string, token string, undo bool) error {
	if len(ids) == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.releases++
	for _, id := range ids {
		row := r.rows[id]
		if row.token != token || row.status != inboxport.StatusProcessing {
			continue
		}
		row.token, row.locked = "", time.Time{}
		if undo {
			row.attempts--
		}
		r.rows[id] = row
	}
	return nil
}

func (r *testInbox) TryClaim(ctx context.Context, id, name string, now, stale time.Time) (bool, string, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.singleClaims++
	row := r.rows[id]
	if row.status == inboxport.StatusCompleted || row.status == inboxport.StatusDead {
		return false, "", row.attempts, nil
	}
	if row.locked.After(stale) {
		return false, "", row.attempts, inboxport.ErrEventInProgress
	}
	row.status, row.token, row.locked = inboxport.StatusProcessing, "single-token", now
	row.attempts++
	r.rows[id] = row
	return true, row.token, row.attempts, nil
}

func (r *testInbox) mark(id, token, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.rows[id]
	if row.token != token {
		return inboxport.ErrLeaseLost
	}
	row.token, row.locked, row.status = "", time.Time{}, status
	r.rows[id] = row
	return nil
}
func (r *testInbox) MarkRetry(ctx context.Context, id, token, reason string) error {
	return r.mark(id, token, inboxport.StatusProcessing)
}
func (r *testInbox) MarkCompleted(ctx context.Context, id, token string, now time.Time) error {
	return r.mark(id, token, inboxport.StatusCompleted)
}
func (r *testInbox) MarkDead(ctx context.Context, id, token, reason string, now time.Time) error {
	return r.mark(id, token, inboxport.StatusDead)
}

type testHandler func(context.Context, eventbus.IncomingEvent) error

func (f testHandler) Handle(ctx context.Context, event eventbus.IncomingEvent) error {
	return f(ctx, event)
}

type testPublisher func(context.Context, eventbus.IntegrationEvent) error

func (f testPublisher) Publish(ctx context.Context, event eventbus.IntegrationEvent) error {
	return f(ctx, event)
}

type testSession struct {
	sarama.ConsumerGroupSession
	ctx    context.Context
	marked []int64
}

func (s *testSession) Context() context.Context { return s.ctx }
func (s *testSession) MarkMessage(message *sarama.ConsumerMessage, metadata string) {
	s.marked = append(s.marked, message.Offset)
}

type testClaim struct {
	sarama.ConsumerGroupClaim
	source chan *sarama.ConsumerMessage
}

func (c testClaim) Messages() <-chan *sarama.ConsumerMessage { return c.source }
func (c testClaim) HighWaterMarkOffset() int64               { return 100 }

func newBatchTest(handler testHandler) (*Consumer, *testInbox, saramaAdapter) {
	repo := &testInbox{rows: make(map[string]testInboxRow)}
	consumer := &Consumer{
		handlers:         map[string]eventbus.Handler{"message": handler},
		inbox:            repo,
		txManager:        repo,
		leaseStaleAfter:  10 * time.Second,
		maxRetries:       3,
		deadLetterSuffix: ".dlq",
		batchSize:        50,
		batchLinger:      5 * time.Millisecond,
	}
	topics, _ := NewTopicRouter(map[string]string{"message": "im.message"})
	return consumer, repo, saramaAdapter{consumer: consumer, topicRouter: topics}
}
func testEvents() []eventbus.IncomingEvent {
	return []eventbus.IncomingEvent{{EventID: "a", Name: "message"}, {EventID: "b", Name: "message"}, {EventID: "c", Name: "message"}}
}

func TestConsumerDeadLettersMessageWithoutEventID(t *testing.T) {
	repo := &testInbox{rows: make(map[string]testInboxRow)}
	var deadLetter eventbus.IntegrationEvent
	consumer := &Consumer{
		handlers:            map[string]eventbus.Handler{"message": testHandler(func(context.Context, eventbus.IncomingEvent) error { return nil })},
		inbox:               repo,
		txManager:           repo,
		deadLetterPublisher: testPublisher(func(_ context.Context, event eventbus.IntegrationEvent) error { deadLetter = event; return nil }),
		deadLetterSuffix:    ".dlq",
	}

	if err := consumer.handle(context.Background(), eventbus.IncomingEvent{Name: "message"}); err != nil {
		t.Fatal(err)
	}
	if deadLetter.EventID == "" || deadLetter.Name != "message.dlq" {
		t.Fatalf("invalid dead-letter event: %+v", deadLetter)
	}
	if len(repo.rows) != 0 {
		t.Fatalf("message without event_id must not create an Inbox row: %+v", repo.rows)
	}
}

func testMessages() []*sarama.ConsumerMessage {
	messages := make([]*sarama.ConsumerMessage, 3)
	for i, id := range []string{"a", "b", "c"} {
		messages[i] = &sarama.ConsumerMessage{Topic: "im.message", Offset: int64(i), Headers: []*sarama.RecordHeader{{Key: []byte("event_id"), Value: []byte(id)}}}
	}
	return messages
}

func TestConsumerBatchSuccessAndOffsets(t *testing.T) {
	var order []string
	_, repo, adapter := newBatchTest(func(ctx context.Context, event eventbus.IncomingEvent) error {
		order = append(order, event.EventID)
		return nil
	})
	session := &testSession{ctx: context.Background()}
	claim := testClaim{source: make(chan *sarama.ConsumerMessage, 3)}
	for _, message := range testMessages() {
		claim.source <- message
	}
	close(claim.source)
	if err := adapter.ConsumeClaim(session, claim); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"a", "b", "c"}) || !reflect.DeepEqual(session.marked, []int64{0, 1, 2}) {
		t.Fatalf("order=%v marked=%v", order, session.marked)
	}
	if repo.claims != 1 || repo.completes != 1 || repo.releases != 0 {
		t.Fatalf("claims=%d completes=%d releases=%d", repo.claims, repo.completes, repo.releases)
	}
}

func TestConsumerBatchConflictRollsBackAndDeduplicates(t *testing.T) {
	var order []string
	_, repo, adapter := newBatchTest(func(ctx context.Context, event eventbus.IncomingEvent) error {
		order = append(order, event.EventID)
		return nil
	})
	repo.rows["b"] = testInboxRow{status: inboxport.StatusCompleted, attempts: 1}
	session := &testSession{ctx: context.Background()}
	if err := adapter.consumeBatch(session, testClaim{}, testMessages()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"a", "c"}) || len(session.marked) != 3 {
		t.Fatalf("order=%v marked=%v", order, session.marked)
	}
	if repo.rows["a"].attempts != 1 {
		t.Fatal("批量冲突未回滚")
	}
}

func TestConsumerBatchFailurePreservesPrefixAndReleasesTail(t *testing.T) {
	router, repo, _ := newBatchTest(func(ctx context.Context, event eventbus.IncomingEvent) error {
		if event.EventID == "b" {
			return errors.New("temporary")
		}
		return nil
	})
	completed, retry, err := router.handleBatch(context.Background(), testEvents())
	if err != nil || retry == nil || completed != 1 {
		t.Fatalf("completed=%d retry=%v err=%v", completed, retry, err)
	}
	if repo.rows["a"].status != inboxport.StatusCompleted || repo.rows["b"].attempts != 1 || repo.rows["c"].attempts != 0 || repo.rows["c"].token != "" {
		t.Fatalf("rows=%+v", repo.rows)
	}
}

func TestConsumerBatchNonRetryableContinuesWithoutRepeatingHandler(t *testing.T) {
	var order, dead []string
	router, repo, adapter := newBatchTest(func(ctx context.Context, event eventbus.IncomingEvent) error {
		order = append(order, event.EventID)
		if event.EventID == "b" {
			return eventbus.NonRetryable(errors.New("bad message"))
		}
		return nil
	})
	router.deadLetterPublisher = testPublisher(func(ctx context.Context, event eventbus.IntegrationEvent) error {
		dead = append(dead, event.EventID)
		return nil
	})
	session := &testSession{ctx: context.Background()}
	if err := adapter.consumeBatch(session, testClaim{}, testMessages()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"a", "b", "c"}) || !reflect.DeepEqual(dead, []string{"b"}) || len(session.marked) != 3 || repo.rows["b"].status != inboxport.StatusDead {
		t.Fatalf("order=%v dead=%v rows=%+v offsets=%v", order, dead, repo.rows, session.marked)
	}
}

func TestConsumerBatchCompletionFailureDoesNotMarkOffsets(t *testing.T) {
	_, repo, adapter := newBatchTest(func(context.Context, eventbus.IncomingEvent) error { return nil })
	repo.completeErr = errors.New("database unavailable")
	session := &testSession{ctx: context.Background()}
	if err := adapter.consumeBatch(session, testClaim{}, testMessages()); err == nil {
		t.Fatal("expected completion failure")
	}
	if len(session.marked) != 0 {
		t.Fatal("未落库就提交 offset")
	}
	for id, row := range repo.rows {
		if row.token != "" || row.attempts != 1 {
			t.Fatalf("id=%s row=%+v", id, row)
		}
	}
}

func TestConsumerBatchRebalanceStopsAndReleasesUnexecutedTail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var order []string
	_, repo, adapter := newBatchTest(func(ctx context.Context, event eventbus.IncomingEvent) error {
		order = append(order, event.EventID)
		cancel()
		return nil
	})
	session := &testSession{ctx: ctx}
	if err := adapter.consumeBatch(session, testClaim{}, testMessages()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if len(order) != 1 || len(session.marked) != 0 || repo.rows["a"].attempts != 1 || repo.rows["b"].attempts != 0 {
		t.Fatalf("order=%v marked=%v rows=%+v", order, session.marked, repo.rows)
	}
}

func TestConsumerBatchRenewalAndLeaseLoss(t *testing.T) {
	for _, lose := range []bool{false, true} {
		t.Run(map[bool]string{false: "renew", true: "lost"}[lose], func(t *testing.T) {
			router, repo, _ := newBatchTest(func(ctx context.Context, event eventbus.IncomingEvent) error {
				if event.EventID != "a" {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(35 * time.Millisecond):
					return nil
				}
			})
			router.leaseStaleAfter = 30 * time.Millisecond
			if lose {
				repo.renewErr = inboxport.ErrLeaseLost
			}
			completed, _, err := router.handleBatch(context.Background(), testEvents())
			if repo.renewals == 0 {
				t.Fatal("没有续期")
			}
			if lose && (!errors.Is(err, inboxport.ErrLeaseLost) || completed != 0) {
				t.Fatalf("completed=%d err=%v", completed, err)
			}
			if !lose && (err != nil || completed != 3) {
				t.Fatalf("completed=%d err=%v", completed, err)
			}
		})
	}
}

func TestCollectPartitionBatchLimitsAndCancellation(t *testing.T) {
	source := make(chan *sarama.ConsumerMessage, 3)
	for _, message := range testMessages() {
		source <- message
	}
	messages, closed, err := collectPartitionBatch(context.Background(), source, 2, time.Hour)
	if err != nil || closed || len(messages) != 2 {
		t.Fatalf("len=%d closed=%t err=%v", len(messages), closed, err)
	}
	messages, closed, err = collectPartitionBatch(context.Background(), source, 2, time.Millisecond)
	if err != nil || closed || len(messages) != 1 {
		t.Fatalf("len=%d closed=%t err=%v", len(messages), closed, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = collectPartitionBatch(ctx, source, 2, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestConsumerBatchRetriesTwiceBeforeContinuingTail(t *testing.T) {
	var order []string
	var attempts []time.Time
	_, repo, adapter := newBatchTest(func(ctx context.Context, event eventbus.IncomingEvent) error {
		order = append(order, event.EventID)
		if event.EventID == "b" {
			attempts = append(attempts, time.Now())
			if len(attempts) < 3 {
				return errors.New("temporary")
			}
		}
		return nil
	})
	session := &testSession{ctx: context.Background()}
	if err := adapter.consumeBatch(session, testClaim{}, testMessages()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"a", "b", "b", "b", "c"}) || len(session.marked) != 3 || repo.rows["b"].attempts != 3 {
		t.Fatalf("order=%v marked=%v rows=%+v", order, session.marked, repo.rows)
	}
	if attempts[1].Sub(attempts[0]) < time.Second || attempts[2].Sub(attempts[1]) < 2*time.Second {
		t.Fatalf("retry delays: %v / %v", attempts[1].Sub(attempts[0]), attempts[2].Sub(attempts[1]))
	}
}

func TestConsumerBatchWaitsForExistingLeaseWithoutSkippingOffset(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	var order []string
	_, repo, adapter := newBatchTest(func(ctx context.Context, event eventbus.IncomingEvent) error {
		order = append(order, event.EventID)
		return nil
	})
	repo.rows["b"] = testInboxRow{status: inboxport.StatusProcessing, token: "other-owner", locked: time.Now(), attempts: 1}
	session := &testSession{ctx: ctx}
	if err := adapter.consumeBatch(session, testClaim{}, testMessages()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"a"}) || !reflect.DeepEqual(session.marked, []int64{0}) || repo.rows["b"].token != "other-owner" {
		t.Fatalf("order=%v marked=%v rows=%+v", order, session.marked, repo.rows)
	}
}

func TestInboxClaimConflictPollingDelay(t *testing.T) {
	var handled int
	router, repo, adapter := newBatchTest(func(context.Context, eventbus.IncomingEvent) error { handled++; return nil })
	router.leaseStaleAfter = 2 * time.Second // 当前逻辑每 200ms 检查一次
	repo.rows["a"] = testInboxRow{status: inboxport.StatusProcessing, token: "other-consumer", locked: time.Now(), attempts: 1}
	message := testMessages()[0]
	claim := testClaim{source: make(chan *sarama.ConsumerMessage, 1)}
	claim.source <- message
	close(claim.source)
	go func() {
		time.Sleep(650 * time.Millisecond)
		repo.mu.Lock()
		row := repo.rows["a"]
		row.locked = time.Now().Add(-3 * time.Second)
		repo.rows["a"] = row
		repo.mu.Unlock()
	}()
	session := &testSession{ctx: context.Background()}
	started := time.Now()
	err := adapter.ConsumeClaim(session, claim)
	elapsed := time.Since(started)
	repo.mu.Lock()
	claimCount := repo.singleClaims
	repo.mu.Unlock()
	t.Logf("active lease released at 650ms; fixed retry interval=%s; claim attempts=%d; delivery resumed after=%s", router.claimRetryInterval(), claimCount, elapsed)
	if err != nil {
		t.Fatal(err)
	}
	if handled != 1 || !reflect.DeepEqual(session.marked, []int64{0}) {
		t.Fatalf("handled=%d marked=%v", handled, session.marked)
	}
	if elapsed < 750*time.Millisecond || elapsed > 1500*time.Millisecond {
		t.Fatalf("unexpected polling delay: %s", elapsed)
	}
	if claimCount < 4 || claimCount > 7 {
		t.Fatalf("unexpected number of claim attempts: %d", claimCount)
	}
}

func TestConsumerBatchDuplicateIDsExecuteOnlyOnce(t *testing.T) {
	var count int
	_, repo, adapter := newBatchTest(func(context.Context, eventbus.IncomingEvent) error { count++; return nil })
	messages := testMessages()
	messages[1].Headers = messages[0].Headers
	session := &testSession{ctx: context.Background()}
	if err := adapter.consumeBatch(session, testClaim{}, messages); err != nil {
		t.Fatal(err)
	}
	if count != 2 || repo.claims != 0 || len(session.marked) != 3 {
		t.Fatalf("count=%d claims=%d marked=%v", count, repo.claims, session.marked)
	}
}

func TestConsumerBatchDeadLetterFailureDoesNotSkipMessage(t *testing.T) {
	router, repo, adapter := newBatchTest(func(ctx context.Context, event eventbus.IncomingEvent) error {
		if event.EventID == "b" {
			return eventbus.NonRetryable(errors.New("bad message"))
		}
		return nil
	})
	router.deadLetterPublisher = testPublisher(func(context.Context, eventbus.IntegrationEvent) error { return errors.New("broker unavailable") })
	session := &testSession{ctx: context.Background()}
	if err := adapter.consumeBatch(session, testClaim{}, testMessages()); err == nil {
		t.Fatal("expected DLQ failure")
	}
	if len(session.marked) != 0 || repo.rows["b"].status == inboxport.StatusDead || repo.rows["c"].attempts != 0 {
		t.Fatalf("marked=%v rows=%+v", session.marked, repo.rows)
	}
}

type countingInboxRepository struct {
	inboxport.InboxRepository
	conflicts *atomic.Int64
}

func (r *countingInboxRepository) WithTx(tx any) inboxport.InboxRepository {
	return &countingInboxRepository{InboxRepository: r.InboxRepository.WithTx(tx), conflicts: r.conflicts}
}

func (r *countingInboxRepository) TryClaim(ctx context.Context, eventID, eventType string, now, staleBefore time.Time) (bool, string, int, error) {
	claimed, token, count, err := r.InboxRepository.TryClaim(ctx, eventID, eventType, now, staleBefore)
	if errors.Is(err, inboxport.ErrEventInProgress) {
		r.conflicts.Add(1)
	}
	return claimed, token, count, err
}

// TestInboxClaimContentionAgainstMySQL 需显式设置 IM_RUN_MYSQL_CONTENTION_TEST=1 后运行。
func TestInboxClaimContentionAgainstMySQL(t *testing.T) {
	if os.Getenv("IM_RUN_MYSQL_CONTENTION_TEST") != "1" {
		t.Skip("设置 IM_RUN_MYSQL_CONTENTION_TEST=1 后运行真实 MySQL 抢占冲突测试")
	}
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = filepath.Join("..", "..", "..", "..", "configs", "config.yaml")
	}
	config := configs.LoadConfig(configPath)
	db, err := gorm.Open(mysql.Open(config.Database.MySQL.DSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	eventID := "mysql-contention-test-" + uuid.NewString()
	if err := db.Where("event_id = ?", eventID).Delete(&model.InboxRecord{}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Where("event_id = ?", eventID).Delete(&model.InboxRecord{}) })

	var conflicts atomic.Int64
	repository := &countingInboxRepository{InboxRepository: mysqlrepo.NewInboxRepository(db), conflicts: &conflicts}
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var handlerCalls atomic.Int64
	handler := testHandler(func(ctx context.Context, event eventbus.IncomingEvent) error {
		if handlerCalls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return nil
	})
	consumer := &Consumer{
		handlers:         map[string]eventbus.Handler{"message": handler},
		inbox:            repository,
		txManager:        persistence.NewGormTxManager(db),
		leaseStaleAfter:  10 * time.Second,
		maxRetries:       3,
		deadLetterSuffix: ".dlq",
	}
	topicRouter, err := NewTopicRouter(map[string]string{"message": "im.message"})
	if err != nil {
		t.Fatal(err)
	}
	event := eventbus.IncomingEvent{EventID: eventID, Name: "message"}
	firstResult := make(chan error, 1)
	go func() { firstResult <- consumer.handle(context.Background(), event) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("第一个处理器未取得 Inbox 租约")
	}
	go func() {
		time.Sleep(1250 * time.Millisecond)
		releaseOnce.Do(func() { close(release) })
	}()

	message := &sarama.ConsumerMessage{Topic: "im.message", Partition: 0, Offset: 10, Headers: []*sarama.RecordHeader{{Key: []byte("event_id"), Value: []byte(eventID)}}}
	claim := testClaim{source: make(chan *sarama.ConsumerMessage, 1)}
	claim.source <- message
	close(claim.source)
	session := &testSession{ctx: context.Background()}
	started := time.Now()
	consumerErr := (saramaAdapter{consumer: consumer, topicRouter: topicRouter}).ConsumeClaim(session, claim)
	elapsed := time.Since(started)
	firstErr := <-firstResult

	t.Logf("真实 MySQL 抢占冲突次数=%d，租约持有=1250ms，轮询间隔=%s，消费者恢复处理耗时=%s，handler 调用次数=%d", conflicts.Load(), consumer.claimRetryInterval(), elapsed, handlerCalls.Load())
	if consumerErr != nil || firstErr != nil {
		t.Fatalf("consumer err=%v first handler err=%v", consumerErr, firstErr)
	}
	if conflicts.Load() != 2 {
		t.Fatalf("预期 2 次 ErrEventInProgress，实际 %d 次", conflicts.Load())
	}
	if elapsed < 1800*time.Millisecond || elapsed > 4*time.Second {
		t.Fatalf("抢占轮询耗时超出预期：%s", elapsed)
	}
	if handlerCalls.Load() != 1 {
		t.Fatalf("幂等状态判断异常，handler 调用次数=%d", handlerCalls.Load())
	}
	if len(session.marked) != 1 || session.marked[0] != message.Offset {
		t.Fatalf("完成后未标记正确 Kafka offset：%v", session.marked)
	}
}
