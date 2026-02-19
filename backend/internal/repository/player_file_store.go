package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"bitdash/backend/internal/usecase"
)

type playerFileStore struct {
	path string
	mu   sync.Mutex
}

// NewPlayerFileStore creates file-based storage for player response snapshots.
func NewPlayerFileStore(path string) usecase.PlayerResponseStore {
	return &playerFileStore{path: path}
}

func (s *playerFileStore) SavePlayerByIDResponse(_ context.Context, _ usecase.PlayerByIDRequest, response *usecase.PlayerByIDResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return os.WriteFile(s.path, payload, 0o644)
}
