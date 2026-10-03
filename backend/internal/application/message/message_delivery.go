package message

import (
	"IM_backend/configs"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	"IM_backend/internal/application/ports/realtime"
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/shared/diagnostics"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"log"
	"sync"
	"time"
)

var ErrLargeRoomMessageBatchFull = errors.New("大群消息批处理队列已满")

type LargeRoomMessageBatchOptions struct {
	Linger     time.Duration
	ShardCount int
	MaxPending int
}

type MessageDelivery struct {
	realtime       realtime.RealtimeDelivery
	roomCache      roomcache.RoomCache
	roomRepository roomrepo.RoomRepository
	config         configs.MessageConfig
	messageBatcher *largeRoomMessageBatcher
}

func NewMessageDelivery(
	delivery realtime.RealtimeDelivery,
	roomRepository roomrepo.RoomRepository,
	roomCache roomcache.RoomCache,
	config configs.MessageConfig,
) *MessageDelivery {
	if config.RoomRealtimeFanoutLimit <= 0 ||
		config.LargeRoomNoticeLingerMilliseconds <= 0 ||
		config.LargeRoomNoticeShardCount <= 0 ||
		config.LargeRoomNoticeMaxPending <= 0 {
		panic("消息投递配置无效：请检查 message.room_realtime_fanout_limit、large_room_notice_linger_milliseconds、large_room_notice_shard_count 和 large_room_notice_max_pending")
	}

	return &MessageDelivery{
		realtime:       delivery,
		roomRepository: roomRepository,
		roomCache:      roomCache,
		config:         config,
		messageBatcher: newLargeRoomMessageBatcher(delivery, roomCache, LargeRoomMessageBatchOptions{
			Linger:     time.Duration(config.LargeRoomNoticeLingerMilliseconds) * time.Millisecond,
			ShardCount: config.LargeRoomNoticeShardCount,
			MaxPending: config.LargeRoomNoticeMaxPending,
		}),
	}
}

func (delivery *MessageDelivery) Deliver(
	ctx context.Context,
	eventType string,
	conversationID string,
	envelope protocol.Envelope,
	event protocol.MessageEvent,
) error {
	switch event.ConvType {
	case protocol.PrivateChat:
		return delivery.deliverPrivateMessage(eventType, envelope)
	case protocol.RoomChat:
		return delivery.deliverRoomMessage(ctx, eventType, conversationID, envelope, event)
	default:
		return ErrUnknownConversationType
	}
}

func (delivery *MessageDelivery) deliverPrivateMessage(
	eventType string,
	envelope protocol.Envelope,
) error {
	return delivery.realtime.DeliverToUser(eventType, envelope.To, envelope.Payload)
}

func (delivery *MessageDelivery) deliverRoomMessage(
	ctx context.Context,
	eventType string,
	roomID string,
	envelope protocol.Envelope,
	event protocol.MessageEvent,
) error {
	startedAt := time.Now()
	roomLookupStartedAt := time.Now()
	room, err := delivery.roomRepository.FindActiveRoom(roomID, int(roomvo.Normal))
	roomLookupDuration := time.Since(roomLookupStartedAt)
	if err != nil {
		diagnostics.Logf("stage=room_delivery message_id=%s seq=%d room_id=%s room_lookup_us=%d outcome=room_lookup_error error=%q",
			event.MessageId, event.Seq, roomID, roomLookupDuration.Microseconds(), err.Error())
		return err
	}
	if room == nil {
		diagnostics.Logf("stage=room_delivery message_id=%s seq=%d room_id=%s room_lookup_us=%d outcome=room_missing",
			event.MessageId, event.Seq, roomID, roomLookupDuration.Microseconds())
		return roomentity.ErrRoomNotFound
	}

	// 弹幕走独立的按视频时间查询接口，不参与普通消息同步缓存和活跃群判断。
	activityLevel := roomcache.RoomActivityNormal
	activityDuration := time.Duration(0)
	onlineSessions := room.MemberCount
	if delivery.roomCache != nil && !event.HasVideoTime {
		activityStartedAt := time.Now()
		if counter, ok := delivery.realtime.(realtime.RoomOnlineSessionCounter); ok {
			if count, countErr := counter.OnlineRoomSessionCount(ctx, roomID); countErr != nil {
				log.Printf("读取房间在线连接数失败，使用成员数保守估算: room=%s err=%v", roomID, countErr)
			} else {
				onlineSessions = count
			}
		}
		level, err := delivery.roomCache.RecordActivityAndGetLevel(ctx, roomID, onlineSessions)
		if err != nil {
			log.Printf("记录房间活跃度失败: room=%s err=%v", roomID, err)
		} else {
			activityLevel = level
		}
		activityDuration = time.Since(activityStartedAt)
	}

	// 成员数是硬阈值；消息频率达到 ACTIVE 后也切换为 PULL。
	// WARN 提前把消息写入 seq ZSET，为客户端稍后拉取预热数据。
	hardPull := room.MemberCount > delivery.config.RoomRealtimeFanoutLimit
	activePull := activityLevel == roomcache.RoomActivityActive
	if hardPull || activePull {
		noticeStartedAt := time.Now()
		err := delivery.deliverLargeRoomNotice(roomID, envelope, event)
		diagnostics.Logf("stage=room_delivery message_id=%s seq=%d room_id=%s room_lookup_us=%d activity_us=%d notice_enqueue_us=%d total_us=%d member_count=%d online_sessions=%d activity_level=%d mode=light_notice outcome=%s",
			event.MessageId, event.Seq, roomID, roomLookupDuration.Microseconds(), activityDuration.Microseconds(), time.Since(noticeStartedAt).Microseconds(),
			time.Since(startedAt).Microseconds(), room.MemberCount, onlineSessions, activityLevel, deliveryOutcome(err))
		return err
	}

	if activityLevel == roomcache.RoomActivityWarn && delivery.roomCache != nil && !event.HasVideoTime {
		if err := delivery.roomCache.AppendRecentMessageSeq(ctx, roomID, event); err != nil {
			log.Printf(
				"写入房间近期消息缓存失败: room=%s seq=%d err=%v",
				roomID,
				event.Seq,
				err,
			)
		}
	}

	fanoutStartedAt := time.Now()
	err = delivery.realtime.DeliverToOnlineRoomMembers(
		eventType,
		roomID,
		envelope.Payload,
		envelope.From,
	)
	diagnostics.Logf("stage=room_delivery message_id=%s seq=%d room_id=%s room_lookup_us=%d activity_us=%d fanout_us=%d total_us=%d member_count=%d online_sessions=%d activity_level=%d mode=fanout outcome=%s",
		event.MessageId, event.Seq, roomID, roomLookupDuration.Microseconds(), activityDuration.Microseconds(), time.Since(fanoutStartedAt).Microseconds(),
		time.Since(startedAt).Microseconds(), room.MemberCount, onlineSessions, activityLevel, deliveryOutcome(err))
	return err
}

// 当房间需要进行轻量推送时，收集本批消息并在 flush 时批量预热缓存。
func (delivery *MessageDelivery) deliverLargeRoomNotice(
	roomID string,
	envelope protocol.Envelope,
	event protocol.MessageEvent,
) error {
	startedAt := time.Now()
	err := delivery.messageBatcher.enqueue(largeRoomNotice{
		ConversationID: roomID,
		SenderID:       envelope.From,
		MessageID:      event.MessageId,
		Seq:            event.Seq,
		Event:          event,
	})
	diagnostics.Logf("stage=large_room_notice_enqueue message_id=%s seq=%d room_id=%s enqueue_us=%d outcome=%s",
		event.MessageId, event.Seq, roomID, time.Since(startedAt).Microseconds(), deliveryOutcome(err))
	return err
}

func deliveryOutcome(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

func (delivery *MessageDelivery) Close(ctx context.Context) {
	delivery.messageBatcher.close(ctx)
}

type largeRoomNotice struct {
	ConversationID string
	SenderID       string
	MessageID      string
	Seq            int64
	Event          protocol.MessageEvent
}

type pendingRoomMessages struct {
	events          []protocol.MessageEvent
	notice          largeRoomNotice
	firstEnqueuedAt time.Time
}

type largeRoomMessageBatcher struct {
	delivery  realtime.RoomMemberDelivery
	roomCache roomcache.RoomCache
	shards    []*largeRoomMessageBatchShard
	done      chan struct{}
	once      sync.Once
}

type largeRoomMessageBatchShard struct {
	mu           sync.Mutex
	pending      map[string]*pendingRoomMessages
	pendingCount int
	maxPending   int
	ticker       *time.Ticker
	parent       *largeRoomMessageBatcher
}

func newLargeRoomMessageBatcher(
	delivery realtime.RoomMemberDelivery,
	roomCache roomcache.RoomCache,
	options LargeRoomMessageBatchOptions,
) *largeRoomMessageBatcher {
	if options.Linger <= 0 || options.ShardCount <= 0 || options.MaxPending <= 0 {
		panic("大群消息批处理配置无效")
	}

	batcher := &largeRoomMessageBatcher{
		delivery:  delivery,
		roomCache: roomCache,
		shards:    make([]*largeRoomMessageBatchShard, options.ShardCount),
		done:      make(chan struct{}),
	}
	shardMax := options.MaxPending / options.ShardCount
	if shardMax <= 0 {
		shardMax = 1
	}
	for i := range batcher.shards {
		shard := &largeRoomMessageBatchShard{
			pending:    make(map[string]*pendingRoomMessages),
			maxPending: shardMax,
			ticker:     time.NewTicker(options.Linger),
			parent:     batcher,
		}
		batcher.shards[i] = shard
		go shard.run()
	}
	return batcher
}

// 根据 roomID 收集消息；同一批最终只保留一个最新 notice。
func (coalescer *largeRoomMessageBatcher) enqueue(notice largeRoomNotice) error {
	if coalescer == nil {
		return nil
	}
	// 大房间只按照会话 ID 推送提示，不再查询房间成员
	key := notice.ConversationID
	return coalescer.shardFor(key).put(key, notice)
}

func (coalescer *largeRoomMessageBatcher) flushAll(ctx context.Context) {
	if coalescer == nil {
		return
	}
	for _, shard := range coalescer.shards {
		shard.flush(ctx)
	}
}

func (coalescer *largeRoomMessageBatcher) close(ctx context.Context) {
	if coalescer == nil {
		return
	}
	coalescer.once.Do(func() {
		close(coalescer.done)
		for _, shard := range coalescer.shards {
			shard.ticker.Stop()
		}
		coalescer.flushAll(ctx)
	})
}

func (coalescer *largeRoomMessageBatcher) shardFor(key string) *largeRoomMessageBatchShard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return coalescer.shards[int(h.Sum32())%len(coalescer.shards)]
}

func (shard *largeRoomMessageBatchShard) put(key string, notice largeRoomNotice) error {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	current, exists := shard.pending[key]
	if shard.pendingCount >= shard.maxPending {
		return ErrLargeRoomMessageBatchFull
	}

	if !exists {
		current = &pendingRoomMessages{firstEnqueuedAt: time.Now()}
		shard.pending[key] = current
	}

	if shard.parent.roomCache != nil && !notice.Event.HasVideoTime {
		current.events = append(current.events, notice.Event)
	}
	shard.pendingCount++
	// 只保留最大的 seq 作为本轮通知的游标，但缓存预热保留本轮全部消息。
	if !exists || current.notice.Seq < notice.Seq {
		current.notice = notice
	}
	return nil
}

func (shard *largeRoomMessageBatchShard) run() {
	for {
		select {
		case <-shard.ticker.C:
			shard.flush(context.Background())
		case <-shard.parent.done:
			return
		}
	}
}

func (shard *largeRoomMessageBatchShard) flush(ctx context.Context) {
	shard.mu.Lock()
	if len(shard.pending) == 0 {
		shard.mu.Unlock()
		return
	}
	pending := make(map[string]*pendingRoomMessages, len(shard.pending))
	for key, roomPending := range shard.pending {
		pending[key] = roomPending
	}
	shard.pending = make(map[string]*pendingRoomMessages)
	shard.pendingCount = 0
	shard.mu.Unlock()

	for _, roomPending := range pending {
		flushStartedAt := time.Now()
		linger := time.Duration(0)
		if !roomPending.firstEnqueuedAt.IsZero() {
			linger = flushStartedAt.Sub(roomPending.firstEnqueuedAt)
		}
		cacheWarmDuration := time.Duration(0)
		if shard.parent.roomCache != nil && len(roomPending.events) > 0 {
			cacheStartedAt := time.Now()
			if err := shard.parent.roomCache.WarmRecentMessageEvents(
				ctx,
				roomPending.notice.ConversationID,
				roomPending.events,
			); err != nil {
				// Redis 只是消息同步的加速层。缓存预热失败时仍然发送 notice，
				// 客户端同步接口会回源 MySQL。
				log.Printf(
					"批量写入房间近期消息缓存失败: room=%s count=%d seq=%d err=%v",
					roomPending.notice.ConversationID,
					len(roomPending.events),
					roomPending.notice.Seq,
					err,
				)
			}
			cacheWarmDuration = time.Since(cacheStartedAt)
		}

		notice := roomPending.notice
		// 将 notice 推送给房间内当前在线的成员；客户端通过 seq 拉取完整消息。
		payload, err := json.Marshal(protocol.MessageNotifyEvent{
			ConversationId: notice.ConversationID,
			MessageId:      notice.MessageID,
			Seq:            notice.Seq,
		})
		if err != nil {
			log.Printf("序列化大群轻量提醒失败：%v", err)
			diagnostics.Logf("stage=large_room_notice_flush room_id=%s seq=%d messages=%d linger_us=%d cache_warm_us=%d outcome=marshal_error error=%q",
				notice.ConversationID, notice.Seq, len(roomPending.events), linger.Microseconds(), cacheWarmDuration.Microseconds(), err.Error())
			continue
		}
		noticeStartedAt := time.Now()
		deliveryErr := shard.parent.delivery.DeliverToOnlineRoomMembers(
			protocol.EventRoomMessageNotice,
			notice.ConversationID,
			payload,
			notice.SenderID,
		)
		if deliveryErr != nil && ctx.Err() == nil {
			log.Printf("投递大群轻量提醒失败：会话=%s seq=%d 错误=%v", notice.ConversationID, notice.Seq, deliveryErr)
		}
		diagnostics.Logf("stage=large_room_notice_flush room_id=%s seq=%d messages=%d linger_us=%d cache_warm_us=%d notice_publish_us=%d total_us=%d outcome=%s",
			notice.ConversationID, notice.Seq, len(roomPending.events), linger.Microseconds(), cacheWarmDuration.Microseconds(),
			time.Since(noticeStartedAt).Microseconds(), time.Since(flushStartedAt).Microseconds(), deliveryOutcome(deliveryErr))
	}
}
