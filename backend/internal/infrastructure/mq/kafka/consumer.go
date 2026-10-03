package kafka

import (
	configs "IM_backend/configs"
	eventbus "IM_backend/internal/application/ports/eventbus"
	inboxport "IM_backend/internal/application/ports/inbox"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	"IM_backend/internal/shared/diagnostics"
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
)

var (
	ErrHandlerNotFound = errors.New("消息处理器不存在")
	ErrInvalidClient   = errors.New("消息队列客户端无效")
	ErrEmptyTopics     = errors.New("消息主题不能为空")
)

type Consumer struct {
	group               sarama.ConsumerGroup
	topicRouter         *TopicRouter
	topics              []string
	retryInterval       time.Duration
	workerPool          *consumerWorkerPool
	handlers            map[string]eventbus.Handler
	inbox               inboxport.InboxRepository
	txManager           txmanager.TxManager
	leaseStaleAfter     time.Duration
	maxRetries          int
	deadLetterPublisher eventbus.Publisher
	deadLetterSuffix    string
	batchSize           int
	batchLinger         time.Duration
}

// claimRetryInterval 根据 Inbox 租约时间推导抢占失败后的检查间隔，避免额外增加配置项。
func (c *Consumer) claimRetryInterval() time.Duration {
	if c == nil || c.leaseStaleAfter <= 0 {
		return time.Second
	}
	interval := c.leaseStaleAfter / 10
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	if interval > time.Second {
		interval = time.Second
	}
	return interval
}

type inboxClaim struct {
	claimed    bool
	lockToken  string
	retryCount int
}

func (c *Consumer) claim(ctx context.Context, message eventbus.IncomingEvent) (inboxClaim, error) {
	if c == nil {
		return inboxClaim{}, ErrHandlerNotFound
	}

	if _, ok := c.handlers[message.Name]; !ok {
		return inboxClaim{}, eventbus.NonRetryable(ErrHandlerNotFound)
	}
	if message.EventID == "" {
		return inboxClaim{}, eventbus.NonRetryable(inboxport.ErrEventIDRequired)
	}
	if c.inbox == nil || c.txManager == nil {
		return inboxClaim{}, errors.New("Inbox 依赖未配置")
	}
	claim := inboxClaim{}
	now := time.Now()
	err := c.txManager.WithinTransaction(ctx, func(tx any) error {
		var err error
		claim.claimed, claim.lockToken, claim.retryCount, err = c.inbox.WithTx(tx).TryClaim(
			ctx,
			message.EventID,
			string(message.Name),
			now,
			now.Add(-c.leaseStaleAfter),
		)
		return err
	})
	if err != nil {
		return claim, err
	}
	return claim, nil
}

func (c *Consumer) execute(ctx context.Context, message eventbus.IncomingEvent) error {
	if c == nil {
		return ErrHandlerNotFound
	}

	handler, ok := c.handlers[message.Name]
	if !ok {
		return eventbus.NonRetryable(ErrHandlerNotFound)
	}
	return handler.Handle(ctx, message)
}

func (c *Consumer) publishDeadLetter(ctx context.Context, message eventbus.IncomingEvent) error {
	if c == nil || c.deadLetterPublisher == nil {
		return errors.New("死信发布器未配置")
	}
	eventID := message.EventID
	if eventID == "" {
		eventID = uuid.NewString()
	}

	if err := c.deadLetterPublisher.Publish(
		ctx,
		eventbus.IntegrationEvent{
			EventID:      eventID,
			Name:         message.Name + c.deadLetterSuffix,
			PartitionKey: string(message.Key),
			Payload:      cloneBytes(message.Payload),
		},
	); err != nil {
		return fmt.Errorf("发布死信消息失败：%w", err)
	}

	return nil
}

func (c *Consumer) handle(ctx context.Context, message eventbus.IncomingEvent) error {
	startedAt := time.Now()
	claimStartedAt := time.Now()
	inboxState, err := c.claim(ctx, message)
	claimDuration := time.Since(claimStartedAt)
	if err != nil {
		diagnostics.Logf("stage=kafka_consumer event_id=%s event_type=%s claim_us=%d total_us=%d outcome=claim_error error=%q",
			message.EventID, message.Name, claimDuration.Microseconds(), time.Since(startedAt).Microseconds(), err.Error())
		if eventbus.IsNonRetryable(err) {
			if publishErr := c.publishDeadLetter(ctx, message); publishErr != nil {
				return publishErr
			}
			return nil
		}

		if errors.Is(err, inboxport.ErrEventInProgress) {
			return err
		}
		log.Printf("消息抢占失败：event_id=%s，error=%v", message.EventID, err)
		return err
	}

	// 已经 completed 或 dead，不需要重复执行业务逻辑。
	if !inboxState.claimed {
		diagnostics.Logf("stage=kafka_consumer event_id=%s event_type=%s claim_us=%d total_us=%d outcome=duplicate_or_terminal",
			message.EventID, message.Name, claimDuration.Microseconds(), time.Since(startedAt).Microseconds())
		return nil
	}

	handlerStartedAt := time.Now()
	handlerErr := c.execute(ctx, message)
	handlerDuration := time.Since(handlerStartedAt)
	if handlerErr != nil {
		diagnostics.Logf("stage=kafka_consumer event_id=%s event_type=%s claim_us=%d handler_us=%d total_us=%d outcome=handler_error error=%q",
			message.EventID, message.Name, claimDuration.Microseconds(), handlerDuration.Microseconds(), time.Since(startedAt).Microseconds(), handlerErr.Error())
		if eventbus.IsNonRetryable(handlerErr) ||
			(c.maxRetries > 0 && inboxState.retryCount >= c.maxRetries) {
			if err := c.publishDeadLetter(ctx, message); err != nil {
				return err
			}
			if err := c.markDead(ctx, message, inboxState.lockToken, handlerErr.Error()); err != nil {
				return fmt.Errorf("标记 Inbox 死信状态失败：%w", err)
			}
			log.Printf(
				"消息进入死信队列：event_id=%s，retry_count=%d，error=%v",
				message.EventID,
				inboxState.retryCount,
				handlerErr,
			)
			return nil
		}

		if err := c.markRetry(ctx, message, inboxState.lockToken, handlerErr.Error()); err != nil {
			return fmt.Errorf("释放 Inbox 重试租约失败：%w", err)
		}

		log.Printf(
			"消息处理失败，释放 Inbox 租约并准备重试：event_id=%s，error=%v",
			message.EventID,
			handlerErr,
		)
		return handlerErr
	}

	markCompletedStartedAt := time.Now()
	if err := c.markCompleted(ctx, message, inboxState.lockToken); err != nil {
		diagnostics.Logf("stage=kafka_consumer event_id=%s event_type=%s claim_us=%d handler_us=%d mark_completed_us=%d total_us=%d outcome=complete_error error=%q",
			message.EventID, message.Name, claimDuration.Microseconds(), handlerDuration.Microseconds(), time.Since(markCompletedStartedAt).Microseconds(), time.Since(startedAt).Microseconds(), err.Error())
		return fmt.Errorf("标记 Inbox 完成状态失败：%w", err)
	}
	diagnostics.Logf("stage=kafka_consumer event_id=%s event_type=%s claim_us=%d handler_us=%d mark_completed_us=%d total_us=%d outcome=completed",
		message.EventID, message.Name, claimDuration.Microseconds(), handlerDuration.Microseconds(), time.Since(markCompletedStartedAt).Microseconds(), time.Since(startedAt).Microseconds())

	return nil
}

func (c *Consumer) handleOnWorker(ctx context.Context, message eventbus.IncomingEvent) error {
	if c == nil || c.workerPool == nil {
		return c.handle(ctx, message)
	}
	done, err := c.workerPool.Submit(ctx, string(message.Key), func(workerCtx context.Context) error {
		return c.handle(workerCtx, message)
	})
	if err != nil {
		return err
	}
	return <-done
}

func (c *Consumer) markRetry(ctx context.Context, message eventbus.IncomingEvent, lockToken, lastError string) error {
	if c == nil || c.inbox == nil || message.EventID == "" || lockToken == "" {
		return errors.New("Inbox 重试状态参数无效")
	}
	return c.inbox.MarkRetry(ctx, message.EventID, lockToken, lastError)
}

func (c *Consumer) markDead(ctx context.Context, message eventbus.IncomingEvent, lockToken, lastError string) error {
	if c == nil || c.inbox == nil || message.EventID == "" || lockToken == "" {
		return errors.New("Inbox 死信状态参数无效")
	}
	return c.inbox.MarkDead(ctx, message.EventID, lockToken, lastError, time.Now())
}

func (c *Consumer) markCompleted(ctx context.Context, message eventbus.IncomingEvent, lockToken string) error {
	if c == nil || c.inbox == nil || message.EventID == "" || lockToken == "" {
		return errors.New("Inbox 完成状态参数无效")
	}
	return c.inbox.MarkCompleted(ctx, message.EventID, lockToken, time.Now())
}

type saramaAdapter struct {
	consumer    *Consumer
	topicRouter *TopicRouter
}

func cloneBytes(data []byte) []byte {
	if data == nil {
		return nil
	}

	copied := make([]byte, len(data))
	copy(copied, data)

	return copied
}

func eventIDFromHeaders(headers []*sarama.RecordHeader) string {
	for _, header := range headers {
		if header != nil && string(header.Key) == "event_id" {
			return string(header.Value)
		}
	}
	return ""
}

func diagnosticPublishAge(headers []*sarama.RecordHeader, now time.Time) time.Duration {
	for _, header := range headers {
		if header == nil || string(header.Key) != diagnosticPublishedAtHeader {
			continue
		}
		publishedAt, err := strconv.ParseInt(string(header.Value), 10, 64)
		if err != nil || publishedAt <= 0 {
			return 0
		}
		return now.Sub(time.Unix(0, publishedAt))
	}
	return 0
}

func (h saramaAdapter) incomingEvent(claim sarama.ConsumerGroupClaim, message *sarama.ConsumerMessage) (eventbus.IncomingEvent, error) {
	name, err := h.topicRouter.EventFor(message.Topic)
	if err != nil {
		return eventbus.IncomingEvent{}, err
	}
	event := eventbus.IncomingEvent{
		EventID: eventIDFromHeaders(message.Headers),
		Name:    name,
		Key:     cloneBytes(message.Key),
		Payload: cloneBytes(message.Value),
	}
	behind := max(claim.HighWaterMarkOffset()-message.Offset-1, 0)
	diagnostics.Logf("stage=kafka_consumer_received event_id=%s topic=%s partition=%d offset=%d partition_records_behind=%d published_age_us=%d",
		event.EventID, message.Topic, message.Partition, message.Offset, behind,
		diagnosticPublishAge(message.Headers, time.Now()).Microseconds())
	return event, nil
}

func (saramaAdapter) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (saramaAdapter) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h saramaAdapter) consumeMessages(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
	initialRetries int,
) error {
	ctx := session.Context()
	claimRetryInterval := h.consumer.claimRetryInterval()
	const maxLocalRetries = 2

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			incomingEvent, err := h.incomingEvent(claim, message)
			if err != nil {
				return err
			}

			localRetryCount := initialRetries
			initialRetries = 0
			for {
				err := h.consumer.handleOnWorker(ctx, incomingEvent)
				if err == nil {
					break
				}

				if errors.Is(err, inboxport.ErrEventInProgress) {
					if err := waitForInboxRetry(ctx, claimRetryInterval); err != nil {
						return err
					}
					continue
				}

				if ctx.Err() != nil {
					return ctx.Err()
				}
				if localRetryCount >= maxLocalRetries {
					return fmt.Errorf(
						"处理消息队列消息失败：主题=%s 分区=%d 偏移量=%d：%w",
						message.Topic,
						message.Partition,
						message.Offset,
						err,
					)
				}

				localRetryCount++
				retryDelay := time.Duration(1<<uint(localRetryCount-1)) * time.Second
				if err := waitForInboxRetry(ctx, retryDelay); err != nil {
					return err
				}
			}

			session.MarkMessage(message, "")
		}
	}
}

func waitForInboxRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func normalizeTopics(topics []string) []string {
	result := make([]string, 0, len(topics))
	seen := make(map[string]struct{}, len(topics))

	for _, topic := range topics {
		if topic == "" {
			continue
		}

		if _, exists := seen[topic]; exists {
			continue
		}

		seen[topic] = struct{}{}
		result = append(result, topic)
	}

	return result
}

func NewConsumer(
	client *Client,
	topics []string,
	topicRouter *TopicRouter,
	handlers map[string]eventbus.Handler,
	inbox inboxport.InboxRepository,
	txManager txmanager.TxManager,
	config configs.KafkaConsumerConfig,
	deadLetterPublisher eventbus.Publisher,
) (*Consumer, error) {
	if client == nil || client.ConsumerGroup == nil {
		return nil, ErrInvalidClient
	}

	validTopics := normalizeTopics(topics)
	if len(validTopics) == 0 {
		return nil, ErrEmptyTopics
	}

	if topicRouter == nil || inbox == nil || txManager == nil || deadLetterPublisher == nil {
		return nil, errors.New("Kafka Consumer 依赖未配置")
	}
	if config.MaxRetries <= 0 ||
		config.LeaseStaleAfterSecs <= 0 ||
		config.ConsumeRetryIntervalSecs <= 0 {
		return nil, errors.New("Kafka Consumer 重试配置无效")
	}
	if config.DeadLetterSuffix == "" {
		return nil, errors.New("Kafka DLQ 后缀未配置")
	}

	registered := make(map[string]eventbus.Handler, len(handlers))
	for eventName, handler := range handlers {
		if eventName != "" && handler != nil {
			registered[eventName] = handler
		}
	}
	if len(registered) == 0 {
		return nil, errors.New("Kafka Consumer 未配置消息处理器")
	}
	consumer := &Consumer{
		group:               client.ConsumerGroup,
		topics:              validTopics,
		topicRouter:         topicRouter,
		retryInterval:       time.Duration(config.ConsumeRetryIntervalSecs) * time.Second,
		workerPool:          newConsumerWorkerPool(config.WorkerCount, config.QueueSize),
		handlers:            registered,
		inbox:               inbox,
		txManager:           txManager,
		leaseStaleAfter:     time.Duration(config.LeaseStaleAfterSecs) * time.Second,
		maxRetries:          config.MaxRetries,
		deadLetterPublisher: deadLetterPublisher,
		deadLetterSuffix:    config.DeadLetterSuffix,
		batchSize:           config.BatchSize,
		batchLinger:         time.Duration(config.BatchLingerMs) * time.Millisecond,
	}
	return consumer, nil
}

func (c *Consumer) Consume(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	c.workerPool.Start(ctx)
	defer c.workerPool.Wait()
	handler := saramaAdapter{
		consumer:    c,
		topicRouter: c.topicRouter,
	}
	for ctx.Err() == nil {
		if err := c.group.Consume(ctx, c.topics, handler); err != nil {
			log.Printf(
				"消费消息主题失败：主题=%v，重试间隔=%s，错误=%v",
				c.topics,
				c.retryInterval,
				err,
			)

			select {
			case <-ctx.Done():
				return context.Cause(ctx)
			case <-time.After(c.retryInterval):
			}
		}
	}
	// 返回上下文取消的具体原因
	return context.Cause(ctx)
}

func (c *Consumer) Close() error {
	if c == nil || c.group == nil {
		return nil
	}

	return c.group.Close()
}

type bufferedClaim struct {
	sarama.ConsumerGroupClaim
	messages chan *sarama.ConsumerMessage
}

func (c bufferedClaim) Messages() <-chan *sarama.ConsumerMessage { return c.messages }

func (h saramaAdapter) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	size, linger := h.consumer.batchSize, h.consumer.batchLinger
	if size <= 0 {
		size = 50
	}
	if linger <= 0 {
		linger = 5 * time.Millisecond
	}
	for {
		messages, closed, err := collectPartitionBatch(session.Context(), claim.Messages(), size, linger)
		if err != nil {
			return err
		}
		if len(messages) > 0 {
			if err := h.consumeBatch(session, claim, messages); err != nil {
				return err
			}
		}
		if closed {
			return nil
		}
	}
}

// 每个 ConsumeClaim 只收集自己的分区，满数量或到达时间窗口即停止收集。
func collectPartitionBatch(ctx context.Context, source <-chan *sarama.ConsumerMessage, size int, linger time.Duration) ([]*sarama.ConsumerMessage, bool, error) {
	var first *sarama.ConsumerMessage
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case message, ok := <-source:
		if !ok {
			return nil, true, nil
		}
		first = message
	}
	messages := []*sarama.ConsumerMessage{first}
	timer := time.NewTimer(linger)
	defer timer.Stop()
	for len(messages) < size {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-timer.C:
			return messages, false, nil
		case message, ok := <-source:
			if !ok {
				return messages, true, nil
			}
			messages = append(messages, message)
		}
	}
	return messages, false, nil
}

func (h saramaAdapter) consumeBatch(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim, messages []*sarama.ConsumerMessage) error {
	ctx := session.Context()
	events := make([]eventbus.IncomingEvent, len(messages))
	for i, message := range messages {
		event, err := h.incomingEvent(claim, message)
		if err != nil {
			return err
		}
		events[i] = event
	}
	completed, retryErr, err := h.consumer.handleBatch(ctx, events)
	if err != nil {
		return fmt.Errorf("批量处理 Inbox 失败：%w", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// 只标记数据库已确认完成的连续前缀，不能跳过失败消息提交后面的 offset。
	for _, message := range messages[:completed] {
		session.MarkMessage(message, "")
	}
	if completed == len(messages) {
		return nil
	}
	initialRetries := 0
	if retryErr != nil {
		if err := waitForInboxRetry(ctx, time.Second); err != nil {
			return err
		}
		initialRetries = 1
	}
	buffer := make(chan *sarama.ConsumerMessage, len(messages)-completed)
	for _, message := range messages[completed:] {
		buffer <- message
	}
	close(buffer)
	return h.consumeMessages(session, bufferedClaim{ConsumerGroupClaim: claim, messages: buffer}, initialRetries)
}

// 新事件批量抢占；已有事件、重复 event_id 和未知处理器走逐条状态机处理。
func (c *Consumer) handleBatch(ctx context.Context, events []eventbus.IncomingEvent) (completed int, retryErr error, err error) {
	if len(events) == 0 {
		return 0, nil, nil
	}
	if c == nil {
		return 0, nil, ErrHandlerNotFound
	}
	if c.inbox == nil || c.txManager == nil {
		return 0, nil, errors.New("Inbox 依赖未配置")
	}
	claims := make([]inboxport.ClaimEvent, len(events))
	ids := make([]string, len(events))
	seen := make(map[string]bool, len(events))
	for i, event := range events {
		if event.EventID == "" || seen[event.EventID] || c.handlers[event.Name] == nil {
			return 0, nil, nil
		}
		seen[event.EventID] = true
		ids[i] = event.EventID
		claims[i] = inboxport.ClaimEvent{EventID: event.EventID, EventType: event.Name}
	}
	return c.processBatch(ctx, events, claims, ids)
}

func (c *Consumer) refreshBatchLease(ctx context.Context, cancel context.CancelCauseFunc, ids []string, token string) func() {
	interval := c.leaseStaleAfter / 3
	if interval <= 0 {
		interval = time.Second
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				renewCtx, finish := context.WithTimeout(ctx, interval)
				err := c.inbox.RenewBatch(renewCtx, ids, token, time.Now())
				finish()
				if err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(stop); <-done }) }
}

type batchWorkerOutcome struct {
	attempted bool
	err       error
	dead      bool
}

type conversationBatch struct {
	key     string
	indices []int
}

func (c *Consumer) processBatch(
	ctx context.Context,
	events []eventbus.IncomingEvent,
	claims []inboxport.ClaimEvent,
	ids []string,
) (completed int, retryErr error, err error) {
	startedAt := time.Now()
	token := uuid.NewString()
	claimStartedAt := time.Now()
	err = c.txManager.WithinTransaction(ctx, func(tx any) error {
		return c.inbox.WithTx(tx).TryClaimBatch(ctx, claims, token, time.Now())
	})
	if errors.Is(err, inboxport.ErrBatchConflict) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	claimDuration := time.Since(claimStartedAt)

	workCtx, cancel := context.WithCancelCause(ctx)
	stopRefresh := c.refreshBatchLease(workCtx, cancel, ids, token)

	outcomes := make([]batchWorkerOutcome, len(events))
	groups := groupBatchByKey(events)
	if c.workerPool == nil {
		indices := make([]int, len(events))
		for i := range indices {
			indices[i] = i
		}
		groups = []conversationBatch{{indices: indices}}
	}
	finalized := false
	defer func() {
		stopRefresh()
		cancel(nil)
		if finalized {
			return
		}
		attemptedIDs := make([]string, 0, len(ids))
		unattemptedIDs := make([]string, 0, len(ids))
		for i, outcome := range outcomes {
			if outcome.attempted {
				attemptedIDs = append(attemptedIDs, ids[i])
			} else {
				unattemptedIDs = append(unattemptedIDs, ids[i])
			}
		}
		cleanupCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer stop()
		if releaseErr := c.inbox.ReleaseBatch(cleanupCtx, attemptedIDs, token, false); releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("释放 Inbox 已执行租约失败：%w", releaseErr))
		}
		if releaseErr := c.inbox.ReleaseBatch(cleanupCtx, unattemptedIDs, token, true); releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("释放 Inbox 批量预占失败：%w", releaseErr))
		}
	}()

	runGroup := func(group conversationBatch) error {
		for _, index := range group.indices {
			if err := workCtx.Err(); err != nil {
				return err
			}
			outcomes[index].attempted = true
			outcomes[index].err = c.execute(workCtx, events[index])
			if outcomes[index].err != nil {
				break
			}
		}
		return nil
	}
	if c.workerPool == nil {
		for _, group := range groups {
			if err := runGroup(group); err != nil {
				return 0, nil, err
			}
		}
	} else {
		tasks := make([]<-chan error, 0, len(groups))
		var submitErr error
		for _, group := range groups {
			group := group
			done, err := c.workerPool.Submit(workCtx, group.key, func(context.Context) error {
				return runGroup(group)
			})
			if err != nil {
				submitErr = err
				break
			}
			tasks = append(tasks, done)
		}
		var workerErr error
		for _, done := range tasks {
			if err := <-done; err != nil && workerErr == nil {
				workerErr = err
			}
		}
		if submitErr != nil {
			return 0, nil, submitErr
		}
		if workerErr != nil {
			return 0, nil, workerErr
		}
	}
	if workCtx.Err() != nil {
		return 0, nil, context.Cause(workCtx)
	}
	stopRefresh()
	if workCtx.Err() != nil {
		return 0, nil, context.Cause(workCtx)
	}

	for i := range outcomes {
		if !outcomes[i].attempted || outcomes[i].err == nil {
			continue
		}
		if !eventbus.IsNonRetryable(outcomes[i].err) && c.maxRetries != 1 {
			continue
		}
		if err := c.publishDeadLetter(ctx, events[i]); err != nil {
			return 0, nil, err
		}
		outcomes[i].dead = true
	}

	completeIDs := make([]string, 0, len(ids))
	unattemptedIDs := make([]string, 0, len(ids))
	for i, outcome := range outcomes {
		switch {
		case !outcome.attempted:
			unattemptedIDs = append(unattemptedIDs, ids[i])
		case outcome.err == nil:
			completeIDs = append(completeIDs, ids[i])
		}
	}
	finalizeStartedAt := time.Now()
	err = c.txManager.WithinTransaction(ctx, func(tx any) error {
		txInbox := c.inbox.WithTx(tx)
		if err := txInbox.CompleteBatch(ctx, completeIDs, token, time.Now()); err != nil {
			return err
		}
		for i, outcome := range outcomes {
			if !outcome.attempted || outcome.err == nil {
				continue
			}
			if outcome.dead {
				if err := txInbox.MarkDead(ctx, ids[i], token, outcome.err.Error(), time.Now()); err != nil {
					return err
				}
				continue
			}
			if err := txInbox.MarkRetry(ctx, ids[i], token, outcome.err.Error()); err != nil {
				return err
			}
		}
		return txInbox.ReleaseBatch(ctx, unattemptedIDs, token, true)
	})
	finalizeDuration := time.Since(finalizeStartedAt)
	if err != nil {
		return 0, nil, err
	}
	finalized = true

	for _, outcome := range outcomes {
		if outcome.attempted && (outcome.err == nil || outcome.dead) {
			completed++
			continue
		}
		if outcome.attempted && outcome.err != nil && retryErr == nil {
			retryErr = outcome.err
		}
		break
	}
	diagnostics.Logf("stage=kafka_consumer_batch size=%d worker_groups=%d completed_prefix=%d retry=%t claim_us=%d finalize_us=%d total_us=%d",
		len(events), len(groups), completed, retryErr != nil, claimDuration.Microseconds(), finalizeDuration.Microseconds(), time.Since(startedAt).Microseconds())
	return completed, retryErr, nil
}

func groupBatchByKey(events []eventbus.IncomingEvent) []conversationBatch {
	groups := make([]conversationBatch, 0, len(events))
	indices := make(map[string]int, len(events))
	for i, event := range events {
		key := string(event.Key)
		if key == "" {
			key = fmt.Sprintf("__empty_key_%d", i)
		}
		groupIndex, ok := indices[key]
		if !ok {
			groupIndex = len(groups)
			indices[key] = groupIndex
			groups = append(groups, conversationBatch{key: key})
		}
		groups[groupIndex].indices = append(groups[groupIndex].indices, i)
	}
	return groups
}

type consumerWorkerTask struct {
	run         func(context.Context) error
	done        chan error
	submittedAt time.Time
}

// consumerWorkerPool 按 Kafka message key 将事件固定路由到一个串行 worker。
type consumerWorkerPool struct {
	shards []chan consumerWorkerTask
	wg     sync.WaitGroup
}

func newConsumerWorkerPool(workerCount, queueSize int) *consumerWorkerPool {
	if workerCount <= 0 {
		workerCount = 16
	}
	if queueSize <= 0 {
		queueSize = 100
	}
	pool := &consumerWorkerPool{shards: make([]chan consumerWorkerTask, workerCount)}
	for i := range pool.shards {
		pool.shards[i] = make(chan consumerWorkerTask, queueSize)
	}
	return pool
}

func (pool *consumerWorkerPool) Start(ctx context.Context) {
	if pool == nil {
		return
	}
	for _, shard := range pool.shards {
		pool.wg.Add(1)
		go func(tasks <-chan consumerWorkerTask) {
			defer pool.wg.Done()
			for {
				select {
				case <-ctx.Done():
					pool.finishQueued(tasks, ctx.Err())
					return
				case task := <-tasks:
					queueWait := time.Since(task.submittedAt)
					diagnostics.Logf("stage=consumer_worker_queue queue_us=%d outcome=started", queueWait.Microseconds())
					err := task.run(ctx)
					task.done <- err
					close(task.done)
				}
			}
		}(shard)
	}
}

func (pool *consumerWorkerPool) finishQueued(tasks <-chan consumerWorkerTask, err error) {
	for {
		select {
		case task := <-tasks:
			task.done <- err
			close(task.done)
		default:
			return
		}
	}
}

func (pool *consumerWorkerPool) Submit(ctx context.Context, key string, run func(context.Context) error) (<-chan error, error) {
	if pool == nil || len(pool.shards) == 0 || run == nil {
		return nil, ErrInvalidClient
	}
	task := consumerWorkerTask{run: run, done: make(chan error, 1), submittedAt: time.Now()}
	shard := pool.shards[messageKeyHash(key)%uint32(len(pool.shards))]
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case shard <- task:
		return task.done, nil
	}
}

func (pool *consumerWorkerPool) Wait() {
	if pool != nil {
		pool.wg.Wait()
	}
}
