package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// TTLCache is a bounded, concurrency-safe in-memory key-value store with expiry.
type TTLCache[K comparable, V any] struct {
	mu       sync.RWMutex
	items    map[K]entry[V]
	ttl      time.Duration
	maxItems int
}

// New creates a new bounded TTL cache.
func New[K comparable, V any](ttl time.Duration, maxItems int) *TTLCache[K, V] {
	if maxItems < 16 {
		maxItems = 16
	}
	return &TTLCache[K, V]{
		items:    make(map[K]entry[V], maxItems),
		ttl:      ttl,
		maxItems: maxItems,
	}
}

// Get retrieves an unexpired value from the cache.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}
	if time.Now().After(it.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		var zero V
		return zero, false
	}
	return it.value, true
}

// Set stores a value with the cache's default TTL, pruning expired/oldest entries if at capacity.
func (c *TTLCache[K, V]) Set(key K, val V) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) >= c.maxItems {
		var oldestKey K
		var oldestTime time.Time
		first := true

		for k, v := range c.items {
			if now.After(v.expiresAt) {
				delete(c.items, k)
				continue
			}
			if first || v.expiresAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.expiresAt
				first = false
			}
		}
		if len(c.items) >= c.maxItems && !first {
			delete(c.items, oldestKey)
		}
	}

	c.items[key] = entry[V]{
		value:     val,
		expiresAt: now.Add(c.ttl),
	}
}
