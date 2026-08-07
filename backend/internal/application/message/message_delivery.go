package message

import (
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

type DeliveryOptions struct {
	RoomRealtimeFanoutLimit           int
	LargeRoomNoticeLingerMilliseconds int
	LargeRoomNoticeShardCount         int
	LargeRoomNoticeMaxPending         int
}

type Delivery struct {
	realtime           realtime.RoomDelivery
	roomMemberCache    roomcache.RoomMemberCache
	roomRepository     roomrepo.RoomRepository
	roomUserRepository roomrepo.RoomUserRepository
	options            DeliveryOptions
	noticeCoalescer    *largeRoomNoticeCoalescer
	singleflight       singleflight.Group
}

func NewDelivery(
	delivery realtime.RoomDelivery,
	roomRepository roomrepo.RoomRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	roomMemberCache roomcache.RoomMemberCache,
	options DeliveryOptions,
) *Delivery {
	if options.RoomRealtimeFanoutLimit <= 0 {
		options.RoomRealtimeFanoutLimit = defaultRoomRealtimeFanoutLimit
	}

	return &Delivery{
		realtime:           delivery,
		roomRepository:     roomRepository,
		roomUserRepository: roomUserRepository,
		roomMemberCache:    roomMemberCache,
		options:            options,
		noticeCoalescer: newLargeRoomNoticeCoalescer(delivery, LargeRoomNoticeCoalescerOptions{
			Linger:     time.Duration(options.LargeRoomNoticeLingerMilliseconds) * time.Millisecond,
			ShardCount: options.LargeRoomNoticeShardCount,
			MaxPending: options.LargeRoomNoticeMaxPending,
		}),
	}
}

func (delivery *Delivery) Deliver(
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

func (delivery *Delivery) deliverPrivateMessage(
	eventType string,
	envelope protocol.Envelope,
) error {
	return delivery.realtime.DeliverToUser(eventType, envelope.To, envelope.Payload)
}

func (delivery *Delivery) deliverRoomMessage(
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

	if room.MemberCount > delivery.options.RoomRealtimeFanoutLimit {
		return delivery.deliverLargeRoomNotice(roomID, envelope, event)
	}

	members, err := delivery.roomMembers(ctx, roomID)
	if err != nil {
		return err
	}
	return delivery.deliverSmallRoomMessage(eventType, members, envelope)
}

func (delivery *Delivery) deliverSmallRoomMessage(
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
func (delivery *Delivery) deliverLargeRoomNotice(
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

func (delivery *Delivery) roomMembers(ctx context.Context, roomID string) ([]string, error) {
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

func (delivery *Delivery) Close(ctx context.Context) {
	delivery.noticeCoalescer.close(ctx)
}

func (delivery *Delivery) FlushLargeRoomNotices(ctx context.Context) {
	delivery.noticeCoalescer.flushAll(ctx)
}

type largeRoomNotice struct {
	ConversationID string
	SenderID       string
	MessageID      string
	Seq            int64
}

type largeRoomNoticeCoalescer struct {
	delivery realtime.RoomDelivery
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
	delivery realtime.RoomDelivery,
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
