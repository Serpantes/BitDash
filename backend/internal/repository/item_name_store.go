package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type ItemMeta struct {
	Name          string      `json:"name"`
	IconAssetName string      `json:"iconAssetName,omitempty"`
	Tier          interface{} `json:"tier,omitempty"`
	RarityStr     string      `json:"rarityStr,omitempty"`
}

type ItemNameStore struct {
	path string
	mu   sync.RWMutex
	data map[string]ItemMeta
}

func NewItemNameStore(path string) (*ItemNameStore, error) {
	s := &ItemNameStore{path: path, data: make(map[string]ItemMeta)}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *ItemNameStore) Get(key string) (ItemMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *ItemNameStore) SetMany(entries map[string]ItemMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range entries {
		s.data[k] = v
	}
	return s.flush()
}

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

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for key, value := range raw {
		var meta ItemMeta
		if err := json.Unmarshal(value, &meta); err == nil && meta.Name != "" {
			s.data[key] = meta
			continue
		}
		var name string
		if err := json.Unmarshal(value, &name); err == nil {
			s.data[key] = ItemMeta{Name: name}
		}
	}
	return nil
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
