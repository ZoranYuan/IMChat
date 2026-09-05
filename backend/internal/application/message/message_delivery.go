package message

import (
	"IM_backend/configs"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	"IM_backend/internal/application/ports/realtime"
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var ErrLargeRoomNoticeCoalescerFull = errors.New("大群消息合并队列已满")

const (
	defaultRoomRealtimeFanoutLimit = 500
	defaultLargeRoomNoticeLinger   = 200 * time.Millisecond
	defaultLargeRoomNoticeShards   = 16
	defaultLargeRoomNoticeMax      = 100000
)

type LargeRoomNoticeCoalescerOptions struct {
	Linger     time.Duration
	ShardCount int
	MaxPending int
}

type MessageDelivery struct {
	realtime           realtime.RealtimeDelivery
	roomMemberCache    roomcache.RoomMemberCache
	roomCache          roomcache.RoomCache
	roomRepository     roomrepo.RoomRepository
	roomUserRepository roomrepo.RoomUserRepository
	config             configs.MessageConfig
	noticeCoalescer    *largeRoomNoticeCoalescer
	singleflight       singleflight.Group
}

func NewMessageDelivery(
	delivery realtime.RealtimeDelivery,
	roomRepository roomrepo.RoomRepository,
	roomCache roomcache.RoomCache,
	roomUserRepository roomrepo.RoomUserRepository,
	roomMemberCache roomcache.RoomMemberCache,
	config configs.MessageConfig,
) *MessageDelivery {
	if config.RoomRealtimeFanoutLimit <= 0 {
		config.RoomRealtimeFanoutLimit = defaultRoomRealtimeFanoutLimit
	}
	if config.LargeRoomNoticeLingerMilliseconds <= 0 {
		config.LargeRoomNoticeLingerMilliseconds = int(defaultLargeRoomNoticeLinger / time.Millisecond)
	}
	if config.LargeRoomNoticeShardCount <= 0 {
		config.LargeRoomNoticeShardCount = defaultLargeRoomNoticeShards
	}
	if config.LargeRoomNoticeMaxPending <= 0 {
		config.LargeRoomNoticeMaxPending = defaultLargeRoomNoticeMax
	}

	return &MessageDelivery{
		realtime:           delivery,
		roomRepository:     roomRepository,
		roomUserRepository: roomUserRepository,
		roomMemberCache:    roomMemberCache,
		roomCache:          roomCache,
		config:             config,
		noticeCoalescer: newLargeRoomNoticeCoalescer(delivery, LargeRoomNoticeCoalescerOptions{
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
	room, err := delivery.roomRepository.FindActiveRoom(roomID, int(roomvo.Normal))
	if err != nil {
		return err
	}
	if room == nil {
		return roomentity.ErrRoomNotFound
	}

	// 弹幕走独立的按视频时间查询接口，不参与普通消息同步缓存和活跃群判断。
	activityLevel := roomcache.RoomActivityNormal
	if delivery.roomCache != nil && !event.HasVideoTime {
		if err := delivery.roomCache.RecordActivity(ctx, roomID); err != nil {
			log.Printf("记录房间活跃度失败: room=%s err=%v", roomID, err)
		} else {
			level, err := delivery.roomCache.ActivateLevel(ctx, roomID)
			if err != nil {
				log.Printf("读取房间活跃度失败: room=%s err=%v", roomID, err)
			} else {
				activityLevel = level
			}
		}
	}

	// 成员数是硬阈值；消息频率达到 ACTIVE 后也切换为 PULL。
	// WARN 提前把消息写入 seq ZSET，为客户端稍后拉取预热数据。
	hardPull := room.MemberCount > delivery.config.RoomRealtimeFanoutLimit
	activePull := activityLevel == roomcache.RoomActivityActive
	if hardPull || activePull {
		if delivery.roomCache != nil && !event.HasVideoTime {
			if err := delivery.roomCache.AppendRecentMessageSeq(ctx, roomID, event); err != nil {
				log.Printf(
					"写入房间近期消息缓存失败: room=%s seq=%d err=%v",
					roomID,
					event.Seq,
					err,
				)
			}
		}
		return delivery.deliverLargeRoomNotice(roomID, envelope, event)
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

	members, err := delivery.roomMembers(ctx, roomID)
	if err != nil {
		return err
	}
	return delivery.deliverSmallRoomMessage(eventType, members, envelope)
}

func (delivery *MessageDelivery) deliverSmallRoomMessage(
	eventType string,
	members []string,
	envelope protocol.Envelope,
) error {
	for _, userID := range members {
		if userID == envelope.From {
			continue
		}
		if err := delivery.realtime.DeliverToUser(eventType, userID, envelope.Payload); err != nil {
			return err
		}
	}
	return nil
}

// 当房间需要进行轻量推送
func (delivery *MessageDelivery) deliverLargeRoomNotice(
	roomID string,
	envelope protocol.Envelope,
	event protocol.MessageEvent,
) error {
	return delivery.noticeCoalescer.enqueue(largeRoomNotice{
		ConversationID: roomID,
		SenderID:       envelope.From,
		MessageID:      event.MessageId,
		Seq:            event.Seq,
	})
}

func (delivery *MessageDelivery) roomMembers(ctx context.Context, roomID string) ([]string, error) {
	if delivery.roomMemberCache != nil {
		members, found, err := delivery.roomMemberCache.GetMemberIDs(ctx, roomID)
		if err != nil {
			log.Printf("读取房间成员缓存失败：房间=%s 错误=%v", roomID, err)
		}
		if found {
			return members, nil
		}
	}

	result, err, _ := delivery.singleflight.Do(roomID, func() (any, error) {
		members, err := delivery.roomUserRepository.ListActiveUserIDs(roomID)
		if err != nil {
			return nil, err
		}
		if delivery.roomMemberCache != nil {
			if err := delivery.roomMemberCache.SetMemberIDs(ctx, roomID, members); err != nil {
				log.Printf("写入房间成员缓存失败：房间=%s 错误=%v", roomID, err)
			}
		}
		return members, nil
	})
	if err != nil {
		return nil, err
	}
	return result.([]string), nil
}

func (delivery *MessageDelivery) Close(ctx context.Context) {
	delivery.noticeCoalescer.close(ctx)
}

func (delivery *MessageDelivery) FlushLargeRoomNotices(ctx context.Context) {
	delivery.noticeCoalescer.flushAll(ctx)
}

type largeRoomNotice struct {
	ConversationID string
	SenderID       string
	MessageID      string
	Seq            int64
}

type largeRoomNoticeCoalescer struct {
	delivery realtime.RoomMemberDelivery
	shards   []*largeRoomNoticeShard
	done     chan struct{}
	once     sync.Once
}

type largeRoomNoticeShard struct {
	mu         sync.Mutex
	pending    map[string]largeRoomNotice
	maxPending int
	ticker     *time.Ticker
	parent     *largeRoomNoticeCoalescer
}

func newLargeRoomNoticeCoalescer(
	delivery realtime.RoomMemberDelivery,
	options LargeRoomNoticeCoalescerOptions,
) *largeRoomNoticeCoalescer {
	if options.Linger <= 0 {
		options.Linger = defaultLargeRoomNoticeLinger
	}
	if options.ShardCount <= 0 {
		options.ShardCount = defaultLargeRoomNoticeShards
	}
	if options.MaxPending <= 0 {
		options.MaxPending = defaultLargeRoomNoticeMax
	}

	coalescer := &largeRoomNoticeCoalescer{
		delivery: delivery,
		shards:   make([]*largeRoomNoticeShard, options.ShardCount),
		done:     make(chan struct{}),
	}
	shardMax := options.MaxPending / options.ShardCount
	if shardMax <= 0 {
		shardMax = 1
	}
	for i := range coalescer.shards {
		shard := &largeRoomNoticeShard{
			pending:    make(map[string]largeRoomNotice),
			maxPending: shardMax,
			ticker:     time.NewTicker(options.Linger),
			parent:     coalescer,
		}
		coalescer.shards[i] = shard
		go shard.run()
	}
	return coalescer
}

// 根据 roomID 进行 notice 的合并
func (coalescer *largeRoomNoticeCoalescer) enqueue(notice largeRoomNotice) error {
	if coalescer == nil {
		return nil
	}
	// 大房间只按照会话 ID 推送提示，不再查询房间成员
	key := notice.ConversationID
	return coalescer.shardFor(key).put(key, notice)
}

func (coalescer *largeRoomNoticeCoalescer) flushAll(ctx context.Context) {
	if coalescer == nil {
		return
	}
	for _, shard := range coalescer.shards {
		shard.flush(ctx)
	}
}

func (coalescer *largeRoomNoticeCoalescer) close(ctx context.Context) {
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

func (coalescer *largeRoomNoticeCoalescer) shardFor(key string) *largeRoomNoticeShard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return coalescer.shards[int(h.Sum32())%len(coalescer.shards)]
}

func (shard *largeRoomNoticeShard) put(key string, notice largeRoomNotice) error {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	current, exists := shard.pending[key]
	if !exists && len(shard.pending) >= shard.maxPending {
		return ErrLargeRoomNoticeCoalescerFull
	}

	// 只保留最大的 seq
	if exists && current.Seq >= notice.Seq {
		return nil
	}
	shard.pending[key] = notice
	return nil
}

func (shard *largeRoomNoticeShard) run() {
	for {
		select {
		case <-shard.ticker.C:
			shard.flush(context.Background())
		case <-shard.parent.done:
			return
		}
	}
}

func (shard *largeRoomNoticeShard) flush(ctx context.Context) {
	shard.mu.Lock()
	if len(shard.pending) == 0 {
		shard.mu.Unlock()
		return
	}
	notices := make([]largeRoomNotice, 0, len(shard.pending))
	for key, notice := range shard.pending {
		notices = append(notices, notice)
		delete(shard.pending, key)
	}
	shard.mu.Unlock()

	// 将 notice 推送给房间内当前在线的成员；客户端通过 seq 拉取完整消息。
	for _, notice := range notices {
		payload, err := json.Marshal(protocol.MessageNotifyEvent{
			ConversationId: notice.ConversationID,
			MessageId:      notice.MessageID,
			Seq:            notice.Seq,
		})
		if err != nil {
			log.Printf("序列化大群轻量提醒失败：%v", err)
			continue
		}
		if err := shard.parent.delivery.DeliverToOnlineRoomMembers(
			protocol.EventRoomMessageNotice,
			notice.ConversationID,
			payload,
			notice.SenderID,
		); err != nil && ctx.Err() == nil {
			log.Printf("投递大群轻量提醒失败：会话=%s seq=%d 错误=%v", notice.ConversationID, notice.Seq, err)
		}
	}
}
