package local

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"
)

// 本地会话版本缓存，更新时的语义是当前操作的会话版本
type VersionEntry struct {
	version  int64
	expireAt int64
}

type lruNode struct {
	key   string
	value VersionEntry
}

type ConversationVersionCache struct {
	mu   sync.RWMutex
	data map[string]*list.Element

	lru *list.List

	maxCap int
	ttl    time.Duration

	cleanupTimeInterval time.Duration
}

func (c *ConversationVersionCache) cleanup() {
	now := time.Now().UnixMilli()

	c.mu.RLock()
	defer func() {
		c.mu.RUnlock()
	}()

	for _, ele := range c.data {
		node := ele.Value.(*lruNode)
		fmt.Println(node.key)
		if now > node.value.expireAt {
			c.removeElement(ele)
		}
	}
}

func (c *ConversationVersionCache) removeElement(e *list.Element) {
	node := e.Value.(*lruNode)
	delete(c.data, node.key)
	c.lru.Remove(e)
}

func (c *ConversationVersionCache) evict() {
	back := c.lru.Back()
	if back == nil {
		return
	}

	c.removeElement(back)
}

func (c *ConversationVersionCache) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(c.cleanupTimeInterval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.cleanup()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func NewConversationVersionTTLCache(ttl time.Duration, maxCap int, cleanupTimeInterval time.Duration) *ConversationVersionCache {
	return &ConversationVersionCache{
		data:                make(map[string]*list.Element),
		lru:                 list.New(),
		ttl:                 ttl,
		maxCap:              maxCap,
		cleanupTimeInterval: cleanupTimeInterval,
	}
}

func (c *ConversationVersionCache) SetVersion(convId string, version int64) {
	now := time.Now()

	c.mu.Lock()
	defer func() {
		c.mu.Unlock()
	}()

	ele, ok := c.data[convId]

	if ok {
		// 已经存在
		c.lru.MoveToFront(ele)
		ele.Value.(*lruNode).value = VersionEntry{
			version:  version,
			expireAt: now.Add(c.ttl).UnixMilli(),
		}
		return
	}

	// 新增
	node := &lruNode{
		key: convId,
		value: VersionEntry{
			version:  version,
			expireAt: now.Add(c.ttl).UnixMilli(),
		},
	}

	ele = c.lru.PushFront(node)
	c.data[convId] = ele

	// 淘汰机制
	if c.lru.Len() > c.maxCap {
		c.evict()
	}
}

func (c *ConversationVersionCache) GetVersion(convId string) (int64, bool) {
	c.mu.RLock()
	ele, ok := c.data[convId]
	c.mu.RUnlock()

	now := time.Now().UnixMilli()

	if !ok {
		return 0, false
	}

	node := ele.Value.(*lruNode)

	if now < node.value.expireAt {
		c.lru.MoveToFront(ele)
		return node.value.version, true
	}

	// 过期了
	c.mu.Lock()
	defer func() {
		c.mu.Unlock()
	}()
	c.removeElement(ele)
	return 0, false
}
