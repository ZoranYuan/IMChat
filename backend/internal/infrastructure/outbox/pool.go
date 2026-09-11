package outbox

import (
	outboxport "IM_backend/internal/application/ports/outbox"
	"context"
	"hash/fnv"
	"log"
	"sync"
)

type OutboxPool struct {
	shards []chan *outboxport.Entry
	wg     sync.WaitGroup
	handle func(context.Context, *outboxport.Entry) error
}

func NewOutboxPool(
	size int,
	queueSize int,
	handle func(context.Context, *outboxport.Entry) error,
) *OutboxPool {
	if size <= 0 {
		size = 10
	}
	if queueSize <= 0 {
		queueSize = 100
	}

	p := &OutboxPool{
		shards: make([]chan *outboxport.Entry, size),
		handle: handle,
	}

	for i := range p.shards {
		p.shards[i] = make(chan *outboxport.Entry, queueSize)
	}

	return p
}

func (op *OutboxPool) Start(ctx context.Context) {
	for _, shard := range op.shards {
		op.wg.Add(1)
		go func(ch <-chan *outboxport.Entry) {
			defer op.wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-ch:
					if !ok {
						return
					}
					if item == nil {
						continue
					}

					if err := op.handle(ctx, item); err != nil {
						log.Printf("处理 Outbox 事件失败：id=%s err=%v", item.ID, err)
					}
				}
			}
		}(shard)
	}
}

func (p *OutboxPool) Wait() {
	p.wg.Wait()
}

func (op *OutboxPool) Submit(ctx context.Context, item *outboxport.Entry) error {
	if item == nil {
		return nil
	}

	index := hashKey(item.MessageKey) % uint32(len(op.shards))

	select {
	case <-ctx.Done():
		return ctx.Err()

	case op.shards[index] <- item:
		return nil
	}
}

func hashKey(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}
