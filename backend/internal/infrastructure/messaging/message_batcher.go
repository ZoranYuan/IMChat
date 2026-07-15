package mq

import (
	realtime "IM_backend/internal/infrastructure/realtime"
	"IM_backend/internal/shared/protocol"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// MessageBatcher accumulates room messages per conversation and flushes
// them in batches. Each conversation is handled by its own lightweight
// goroutine (actor) so that locks are unnecessary on the hot path.
//
// Idle actors exit after idleTimeout to avoid unbounded goroutine growth.
type MessageBatcher struct {
	dispatch    realtime.Gateway
	batchSize   int
	flushWindow time.Duration
	idleTimeout time.Duration

	// sync.Map: conversationId → chan msgEnvelope
	//
	// sync.Map is a good fit here because the key space is large (one per
	// conversation) and reads dominate (lookups happen on every incoming
	// message, inserts only when an actor is cold-started).
	actors sync.Map
}

type msgEnvelope struct {
	Topic   string
	Payload []byte
	Members []string
}

// NewMessageBatcher creates a batcher.
//
//	batchSize  – flush immediately once this many messages are buffered.
//	flushWindow – max time to hold a message before flushing.
//	idleTimeout – close an actor after this period of inactivity.
func NewMessageBatcher(dispatch realtime.Gateway, batchSize int, flushWindow, idleTimeout time.Duration) *MessageBatcher {
	return &MessageBatcher{
		dispatch:    dispatch,
		batchSize:   batchSize,
		flushWindow: flushWindow,
		idleTimeout: idleTimeout,
	}
}

// Add enqueues a room message. Thread-safe.
func (b *MessageBatcher) Add(topic, conversationId string, memberIDs []string, event protocol.MessageEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	env := msgEnvelope{Topic: topic, Payload: payload, Members: memberIDs}

	ch := b.getOrCreateActor(conversationId)
	ch <- env
	return nil
}

// getOrCreateActor returns the actor channel for the given conversation,
// starting a new goroutine if one does not exist.
func (b *MessageBatcher) getOrCreateActor(conversationId string) chan<- msgEnvelope {
	if v, ok := b.actors.Load(conversationId); ok {
		return v.(chan msgEnvelope)
	}

	ch := make(chan msgEnvelope, b.batchSize)
	actual, _ := b.actors.LoadOrStore(conversationId, ch)
	loaded := actual.(chan msgEnvelope)

	// We lost the race — another goroutine stored first. Use theirs.
	if loaded != ch {
		return loaded
	}

	// We won the race — start the actor.
	go b.runActor(conversationId, ch)
	return ch
}

// runActor is the per-conversation goroutine. It lives until idleTimeout
// elapses without receiving a message.
func (b *MessageBatcher) runActor(conversationId string, ch <-chan msgEnvelope) {
	buf := make([]msgEnvelope, 0, b.batchSize)
	timer := time.NewTimer(b.flushWindow)
	if !timer.Stop() {
		<-timer.C
	}

	flush := func() {
		if len(buf) == 0 {
			return
		}
		b.flushBuffer(conversationId, buf)
		buf = buf[:0]
	}

	for {
		select {
		case env, ok := <-ch:
			if !ok {
				flush()
				return
			}

			buf = append(buf, env)

			if len(buf) == 1 {
				timer.Reset(b.flushWindow)
			}

			if len(buf) >= b.batchSize {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				flush()
			}

		case <-timer.C:
			flush()

		case <-time.After(b.idleTimeout):
			// No messages for a while — shut down this actor.
			b.actors.Delete(conversationId)
			flush()
			return
		}
	}
}

// flushBuffer pushes every buffered message to every member of the room.
func (b *MessageBatcher) flushBuffer(conversationId string, buf []msgEnvelope) {
	for _, env := range buf {
		for _, uid := range env.Members {
			if err := b.dispatch.DeliverToUser(env.Topic, uid, env.Payload); err != nil {
				log.Printf("batcher: push to %s failed: %v", uid, err)
			}
		}
	}
}

// Shutdown gracefully drains all actors. Call before process exit.
func (b *MessageBatcher) Shutdown() {
	b.actors.Range(func(key, value any) bool {
		ch := value.(chan msgEnvelope)
		close(ch)
		return true
	})
}
