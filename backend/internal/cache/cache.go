package cache

import (
	"sync"
	"time"
)

type entry struct {
	data      []byte
	expiresAt time.Time
}

type Cache struct {
	mu  sync.RWMutex
	m   map[string]entry
	ttl time.Duration
}

func New(ttl time.Duration) *Cache {
	c := &Cache{m: make(map[string]entry), ttl: ttl}
	go c.evict()
	return c
}

func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.data, true
}

func (c *Cache) Set(key string, data []byte) {
	c.mu.Lock()
	c.m[key] = entry{data: data, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

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
