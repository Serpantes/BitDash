package cache

import (
	"sync"
	"time"
)

type entry struct {
	data      []byte
	expiresAt time.Time
}

// Cache is a simple in-memory TTL cache safe for concurrent use.
type Cache struct {
	mu  sync.RWMutex
	m   map[string]entry
	ttl time.Duration
}

// New creates a Cache with the given TTL and starts a background eviction goroutine.
func New(ttl time.Duration) *Cache {
	c := &Cache{m: make(map[string]entry), ttl: ttl}
	go c.evict()
	return c
}

// Get returns cached bytes for key. Returns nil, false if missing or expired.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.data, true
}

// Set stores data under key with the configured TTL.
func (c *Cache) Set(key string, data []byte) {
	c.mu.Lock()
	c.m[key] = entry{data: data, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// evict periodically removes expired entries.
func (c *Cache) evict() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, e := range c.m {
			if now.After(e.expiresAt) {
				delete(c.m, k)
			}
		}
		c.mu.Unlock()
	}
}
