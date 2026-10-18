package repository

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDashboardStoreSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard.json")
	s, err := NewDashboardStore(path)
	if err != nil {
		t.Fatal(err)
	}
	empty, _ := s.Load()
	if empty.Rev != 0 || string(empty.Blocks) != "[]" {
		t.Fatalf("empty store = %+v", empty)
	}

	saved, err := s.Save(0, json.RawMessage(`[{"inventoryId":"a"}]`), false)
	if err != nil || saved.Rev != 1 {
		t.Fatalf("save = %+v, %v", saved, err)
	}

	reopened, err := NewDashboardStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := reopened.Load()
	if got.Rev != 1 || string(got.Blocks) != `[{"inventoryId":"a"}]` {
		t.Fatalf("reloaded = %+v", got)
	}
}

func TestDashboardStoreRejectsStaleRevision(t *testing.T) {
	s, _ := NewDashboardStore(filepath.Join(t.TempDir(), "dashboard.json"))
	s.Save(0, json.RawMessage(`[{"inventoryId":"a"},{"inventoryId":"b"}]`), false)

	current, err := s.Save(0, json.RawMessage(`[]`), false)
	if !errors.Is(err, ErrDashboardConflict) {
		t.Fatalf("stale save err = %v", err)
	}
	if current.Rev != 1 {
		t.Fatalf("conflict should return current state, got %+v", current)
	}
	got, _ := s.Load()
	if string(got.Blocks) != `[{"inventoryId":"a"},{"inventoryId":"b"}]` {
		t.Fatalf("stale save overwrote blocks: %s", got.Blocks)
	}
}

func TestDashboardStoreBacksUpBeforeRemovingBlocks(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewDashboardStore(filepath.Join(dir, "dashboard.json"))
	s.Save(0, json.RawMessage(`[{"inventoryId":"a"},{"inventoryId":"b"}]`), false)
	if _, err := s.Save(1, json.RawMessage(`[]`), false); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "dashboard-backups"))
	if len(entries) != 1 {
		t.Fatalf("backups = %d, want 1", len(entries))
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "dashboard-backups", entries[0].Name()))
	var backup DashboardState
	json.Unmarshal(raw, &backup)
	if backup.Rev != 1 {
		t.Fatalf("backup = %+v", backup)
	}
}

func TestDashboardStoreRejectsNonArray(t *testing.T) {
	s, _ := NewDashboardStore(filepath.Join(t.TempDir(), "dashboard.json"))
	if _, err := s.Save(0, json.RawMessage(`{"a":1}`), false); err == nil {
		t.Fatal("expected error for non-array blocks")
	}
}
