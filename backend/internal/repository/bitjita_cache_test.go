package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileCacheSetGet(t *testing.T) {
	root := filepath.Join(os.TempDir(), "bitdash-file-cache-test")
	defer os.RemoveAll(root)

	c, err := NewFileCache(root)
	if err != nil {
		t.Fatalf("NewFileCache: %v", err)
	}

	if err := c.Set("items_1.json", []byte("{\"ok\":true}")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	data, ok := c.Get("items_1.json")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(data) != "{\"ok\":true}" {
		t.Fatalf("unexpected data: %s", string(data))
	}
}

func TestBitjitaCachePolicy(t *testing.T) {
	if got := BitjitaCachePolicy("/items/5020003", ""); got != "permanent" {
		t.Fatalf("item detail: got %q", got)
	}
	if got := BitjitaCachePolicy("/items", "q=plank"); got != "permanent" {
		t.Fatalf("item search: got %q", got)
	}
	if got := BitjitaCachePolicy("/status", ""); got != "live" {
		t.Fatalf("status: got %q", got)
	}
	if got := BitjitaCachePolicy("/players/1/stats", ""); got != "live" {
		t.Fatalf("player: got %q", got)
	}
}

func TestIconAssetPathCandidatesKeepsGeneratedIconsPathFirst(t *testing.T) {
	got := iconAssetPathCandidates("GeneratedIcons/Items/KeyMold")
	want := []string{"/GeneratedIcons/Items/KeyMold.webp"}

	if len(got) != len(want) {
		t.Fatalf("expected %d candidates, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}
