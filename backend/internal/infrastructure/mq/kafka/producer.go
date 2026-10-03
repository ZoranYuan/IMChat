package kafka

import (
	"IM_backend/configs"
	eventbus "IM_backend/internal/application/ports/eventbus"
	outboxport "IM_backend/internal/application/ports/outbox"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	"IM_backend/internal/shared/diagnostics"
	"context"
	"errors"
	"hash/fnv"
	"log"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

type Producer struct {
	client     *Client
	router     *TopicRouter
	txManager  txmanager.TxManager
	repository outboxport.OutboxRepository
	options    configs.KafkaProducerConfig
	pool       *producerWorkerPool
	wake       chan time.Time
}

const diagnosticPublishedAtHeader = "im_diag_published_at_unix_nano"

func NewProducer(
	client *Client,
	router *TopicRouter,
	txManager txmanager.TxManager,
	repository outboxport.OutboxRepository,
	options configs.KafkaProducerConfig,
) *Producer {
	return &Producer{
		client:     client,
		router:     router,
		txManager:  txManager,
		repository: repository,
		options:    normalizeProducerConfig(options),
		wake:       make(chan time.Time, 1),
	}
}

func normalizeProducerConfig(options configs.KafkaProducerConfig) configs.KafkaProducerConfig {
	if options.ClaimBatchSize <= 0 {
		options.ClaimBatchSize = 100
	}
	if options.WorkerCount <= 0 {
		options.WorkerCount = 10
	}
	if options.QueueSize <= 0 {
		options.QueueSize = 300
	}
	if options.MarkSentBatchSize <= 0 {
		options.MarkSentBatchSize = 32
	}
	if options.MarkSentBatchLingerMs <= 0 {
		options.MarkSentBatchLingerMs = 5
	}
	if options.PollIntervalSeconds <= 0 {
		options.PollIntervalSeconds = 2
	}
	if options.StaleAfterSeconds <= 0 {
		options.StaleAfterSeconds = 30
	}
	if options.BaseRetryWaitSeconds <= 0 {
		options.BaseRetryWaitSeconds = 2
	}
	if options.WorkerMaxRetries <= 0 {
		options.WorkerMaxRetries = 10
	}
	return options
}

// Produce 将单个事件交给 Kafka；取库、分片调度和状态维护由 Producer 自身负责。
func (p *Producer) Produce(ctx context.Context, event eventbus.IntegrationEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.EventID == "" {
		return errors.New("Kafka 事件缺少 event_id")
	}
	targetTopic, err := p.router.TopicFor(event.Name)
	if err != nil {
		return err
	}
	message := &sarama.ProducerMessage{
		Topic: targetTopic,
		Key:   sarama.StringEncoder(event.PartitionKey),
		Value: sarama.ByteEncoder(event.Payload),
	}
	if diagnostics.Enabled() {
		message.Headers = append(message.Headers, sarama.RecordHeader{
			Key:   []byte(diagnosticPublishedAtHeader),
			Value: []byte(strconv.FormatInt(time.Now().UnixNano(), 10)),
		})
	}
	message.Headers = append(message.Headers, sarama.RecordHeader{
		Key:   []byte("event_id"),
		Value: []byte(event.EventID),
	})
	_, _, err = p.client.Producer.SendMessage(message)
	return err
}

// Publish 让 Producer 同时可作为 Consumer 的死信发布器。
func (p *Producer) Publish(ctx context.Context, event eventbus.IntegrationEvent) error {
	return p.Produce(ctx, event)
}

// Notify 在业务事务提交后唤醒 Producer，容量为 1 的 channel 会合并密集通知。
func (p *Producer) Notify() {
	if p == nil {
		return
	}
	select {
	case p.wake <- time.Now():
	default:
	}
}

// Start 持续领取待投递记录并交给按消息 key 分片的 Producer worker pool。
func (p *Producer) Start(ctx context.Context) {
	p.pool = newProducerWorkerPool(
		p.options.WorkerCount,
		p.options.QueueSize,
		p.options.MarkSentBatchSize,
		time.Duration(p.options.MarkSentBatchLingerMs)*time.Millisecond,
		p.dispatchBatch,
	)
	p.pool.Start(ctx)
	log.Printf("Kafka Producer 已启动：workers=%d queueSize=%d", p.options.WorkerCount, p.options.QueueSize)
	defer p.pool.Wait()

	ticker := time.NewTicker(time.Duration(p.options.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()
	drain := func(source string, triggeredAt time.Time) {
		drainStartedAt := time.Now()
		diagnostics.Logf("stage=producer_wakeup source=%s signal_wait_us=%d", source, drainStartedAt.Sub(triggeredAt).Microseconds())
		firstClaim := true
		for ctx.Err() == nil {
			processed, err := p.producePendingOnce(ctx, source, drainStartedAt, firstClaim)
			firstClaim = false
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("Kafka Producer 领取待投递任务失败：%v", err)
				}
				return
			}
			if !processed {
				return
			}
		}
	}
	drain("startup", time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case tickAt := <-ticker.C:
			drain("poll", tickAt)
		case notifiedAt := <-p.wake:
			drain("notify", notifiedAt)
		}
	}
}

func (p *Producer) producePendingOnce(ctx context.Context, trigger string, drainStartedAt time.Time, firstClaim bool) (bool, error) {
	if p.txManager == nil || p.repository == nil || p.client == nil || p.client.Producer == nil {
		return false, errors.New("Kafka Producer 依赖未配置")
	}
	var batch []*outboxport.Entry
	now := time.Now()
	claimStartedAt := time.Now()
	err := p.txManager.WithinTransaction(ctx, func(tx any) error {
		items, err := p.repository.WithTx(tx).ClaimPending(
			ctx,
			now,
			now.Add(-time.Duration(p.options.StaleAfterSeconds)*time.Second),
			p.options.ClaimBatchSize,
		)
		if err != nil {
			return err
		}
		batch = items
		return nil
	})
	claimDuration := time.Since(claimStartedAt)
	drainElapsed := time.Since(drainStartedAt)
	if err != nil {
		diagnostics.Logf("stage=producer_claim_result source=%s first_in_drain=%t outcome=error drain_elapsed_us=%d claim_us=%d error=%q", trigger, firstClaim, drainElapsed.Microseconds(), claimDuration.Microseconds(), err.Error())
		return false, err
	}
	diagnostics.Logf("stage=producer_claim_result source=%s rows=%d first_in_drain=%t drain_elapsed_us=%d claim_us=%d outcome=ok", trigger, len(batch), firstClaim, drainElapsed.Microseconds(), claimDuration.Microseconds())
	var submitDuration time.Duration
	for _, item := range batch {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}
		if item != nil && !item.CreatedAt.IsZero() {
			diagnostics.Logf("stage=producer_claim_item event_id=%s event_type=%s created_age_ms=%d", item.ID, item.EventType, time.Since(item.CreatedAt).Milliseconds())
		}
		submitStartedAt := time.Now()
		if err := p.pool.Submit(ctx, item); err != nil {
			submitDuration += time.Since(submitStartedAt)
			return false, err
		}
		submitDuration += time.Since(submitStartedAt)
	}
	diagnostics.Logf("stage=producer_claim source=%s rows=%d claim_us=%d worker_submit_us=%d outcome=ok", trigger, len(batch), claimDuration.Microseconds(), submitDuration.Microseconds())
	return len(batch) > 0, nil
}

func (p *Producer) dispatchBatch(ctx context.Context, tasks []producerTask) error {
	if len(tasks) == 0 {
		return nil
	}
	type sentGroup struct {
		lockToken string
		ids       []string
	}
	sentGroups := make([]sentGroup, 0, len(tasks))
	groupIndexes := make(map[string]int, len(tasks))
	var dispatchErr error

	for _, task := range tasks {
		item := task.item
		if item == nil {
			continue
		}
		startedAt := time.Now()
		createdAge := time.Duration(0)
		if !item.CreatedAt.IsZero() {
			createdAge = startedAt.Sub(item.CreatedAt)
		}
		publishStartedAt := time.Now()
		publishErr := p.Produce(ctx, eventbus.IntegrationEvent{
			EventID:      item.ID,
			Name:         item.EventType,
			PartitionKey: item.MessageKey,
			Payload:      item.Payload,
		})
		if publishErr != nil {
			diagnostics.Logf("stage=producer_publish event_id=%s event_type=%s created_age_ms=%d worker_queue_us=%d publish_us=%d outcome=retry error=%q", item.ID, item.EventType, createdAge.Milliseconds(), time.Since(task.submittedAt).Microseconds(), time.Since(publishStartedAt).Microseconds(), publishErr.Error())
			if err := p.markRetry(ctx, item, publishErr.Error()); err != nil {
				dispatchErr = errors.Join(dispatchErr, err)
			}
			continue
		}

		diagnostics.Logf("stage=producer_publish event_id=%s event_type=%s created_age_ms=%d worker_queue_us=%d publish_us=%d outcome=published_pending_mark_sent", item.ID, item.EventType, createdAge.Milliseconds(), time.Since(task.submittedAt).Microseconds(), time.Since(publishStartedAt).Microseconds())
		groupIndex, ok := groupIndexes[item.LockToken]
		if !ok {
			groupIndex = len(sentGroups)
			groupIndexes[item.LockToken] = groupIndex
			sentGroups = append(sentGroups, sentGroup{lockToken: item.LockToken})
		}
		sentGroups[groupIndex].ids = append(sentGroups[groupIndex].ids, item.ID)
	}

	for _, group := range sentGroups {
		startedAt := time.Now()
		err := p.repository.MarkSentBatch(ctx, group.ids, group.lockToken, startedAt)
		diagnostics.Logf("stage=producer_mark_sent_batch size=%d mark_sent_us=%d outcome=%s", len(group.ids), time.Since(startedAt).Microseconds(), producerOutcome(err))
		if err != nil {
			dispatchErr = errors.Join(dispatchErr, err)
			log.Printf("批量更新 Outbox 已发送状态失败：size=%d err=%v", len(group.ids), err)
		}
	}
	return dispatchErr
}

func (p *Producer) markRetry(ctx context.Context, item *outboxport.Entry, lastError string) error {
	if item.RetryCount >= p.options.WorkerMaxRetries {
		return p.repository.MarkDead(ctx, item.ID, item.LockToken, lastError)
	}
	retryDelay := min(time.Duration(float64(time.Duration(p.options.BaseRetryWaitSeconds)*time.Second)*math.Pow(2, float64(item.RetryCount))), 5*time.Minute)
	return p.repository.MarkRetry(ctx, item.ID, item.LockToken, time.Now().Add(retryDelay), lastError)
}

func producerOutcome(err error) string {
	if err != nil {
		return "error"
	}
	return "sent"
}

type producerWorkerPool struct {
	shards      []chan producerTask
	wg          sync.WaitGroup
	batchSize   int
	batchLinger time.Duration
	handle      func(context.Context, []producerTask) error
}

type producerTask struct {
	item        *outboxport.Entry
	submittedAt time.Time
}

func newProducerWorkerPool(
	workerCount, queueSize, batchSize int,
	batchLinger time.Duration,
	handle func(context.Context, []producerTask) error,
) *producerWorkerPool {
	if batchSize <= 0 {
		batchSize = 32
	}
	if batchLinger <= 0 {
		batchLinger = 5 * time.Millisecond
	}
	pool := &producerWorkerPool{
		shards:      make([]chan producerTask, workerCount),
		batchSize:   batchSize,
		batchLinger: batchLinger,
		handle:      handle,
	}
	for i := range pool.shards {
		pool.shards[i] = make(chan producerTask, queueSize)
	}
	return pool
}

func (pool *producerWorkerPool) Start(ctx context.Context) {
	for _, shard := range pool.shards {
		pool.wg.Add(1)
		go func(tasks <-chan producerTask) {
			defer pool.wg.Done()
			for {
				var first producerTask
				select {
				case <-ctx.Done():
					return
				case task, ok := <-tasks:
					if !ok {
						return
					}
					first = task
				}

				batch, running := pool.collectBatch(ctx, tasks, first)
				if !running {
					return
				}
				if err := pool.handle(ctx, batch); err != nil {
					log.Printf("Producer 批量处理待投递事件失败：size=%d err=%v", len(batch), err)
				}
			}
		}(shard)
	}
}

func (pool *producerWorkerPool) collectBatch(
	ctx context.Context,
	tasks <-chan producerTask,
	first producerTask,
) ([]producerTask, bool) {
	batch := make([]producerTask, 1, pool.batchSize)
	batch[0] = first
	timer := time.NewTimer(pool.batchLinger)
	defer timer.Stop()
	for len(batch) < pool.batchSize {
		select {
		case <-ctx.Done():
			return nil, false
		case task, ok := <-tasks:
			if !ok {
				return batch, true
			}
			batch = append(batch, task)
		case <-timer.C:
			return batch, true
		}
	}
	return batch, true
}

func (pool *producerWorkerPool) Wait() { pool.wg.Wait() }

func (pool *producerWorkerPool) Submit(ctx context.Context, item *outboxport.Entry) error {
	if item == nil {
		return nil
	}
	index := messageKeyHash(item.MessageKey) % uint32(len(pool.shards))
	queue := pool.shards[index]
	queueBefore := len(queue)
	submitStartedAt := time.Now()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case queue <- producerTask{item: item, submittedAt: time.Now()}:
		diagnostics.Logf("stage=producer_submit shard=%d queue_before=%d queue_after=%d queue_capacity=%d submit_wait_us=%d", index, queueBefore, len(queue), cap(queue), time.Since(submitStartedAt).Microseconds())
		return nil
	}
}

func messageKeyHash(key string) uint32 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return hash.Sum32()
}
