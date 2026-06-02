package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ItemNameStore caches item/cargo display names keyed as "i:{id}" or "c:{id}".
// It is loaded from disk on creation and flushed to disk on every write.
type ItemNameStore struct {
	path string
	mu   sync.RWMutex
	data map[string]string
}

// NewItemNameStore opens (or creates) the JSON file at path and loads existing names.
func NewItemNameStore(path string) (*ItemNameStore, error) {
	s := &ItemNameStore{
		path: path,
		data: make(map[string]string),
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

// Get returns (name, true) if the key is cached.
func (s *ItemNameStore) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// SetMany adds multiple entries and flushes the store to disk.
func (s *ItemNameStore) SetMany(entries map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range entries {
		s.data[k] = v
	}
	return s.flush()
}

// ItemKey builds the cache key used by ItemNameStore.
// itemType 1 = cargo ("c:{id}"), anything else = item ("i:{id}").
func ItemKey(itemType int, id int64) string {
	if itemType == 1 {
		return fmt.Sprintf("c:%d", id)
	}
	return fmt.Sprintf("i:%d", id)
}

func (s *ItemNameStore) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &s.data)
}

func (s *ItemNameStore) flush() error {
	dir := filepath.Dir(s.path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}
