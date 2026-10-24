package repository

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"bitdash/backend/internal/cache"
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
	if got := BitjitaCachePolicy("/static/experience/levels.json", ""); got != "permanent" {
		t.Fatalf("static levels: got %q", got)
	}
}

func TestBitjitaUpstreamPath(t *testing.T) {
	if got := bitjitaUpstreamPath("/players/1/skill-rankings"); got != "/api/players/1/skill-rankings" {
		t.Fatalf("api path: got %q", got)
	}
	if got := bitjitaUpstreamPath("/static/experience/levels.json"); got != "/static/experience/levels.json" {
		t.Fatalf("static path: got %q", got)
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

func TestBitjitaGetAPIFreshBypassesLiveCache(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		fmt.Fprintf(w, `{"hit":%d}`, hits)
	}))
	defer server.Close()

	b, err := NewBitjitaCache(server.URL, t.TempDir(), cache.New(time.Minute))
	if err != nil {
		t.Fatalf("NewBitjitaCache: %v", err)
	}

	first, _, _, _ := b.GetAPI("/leaderboard/skills", "sortBy=2")
	cached, _, status, _ := b.GetAPI("/leaderboard/skills", "sortBy=2")
	if status != "HIT" || string(cached) != string(first) || hits != 1 {
		t.Fatalf("expected cached response, got %q status %q after %d hits", cached, status, hits)
	}

	fresh, _, status, _ := b.GetAPIFresh("/leaderboard/skills", "sortBy=2")
	if status != "MISS" || string(fresh) != `{"hit":2}` {
		t.Fatalf("expected upstream response, got %q status %q", fresh, status)
	}

	after, _, _, _ := b.GetAPI("/leaderboard/skills", "sortBy=2")
	if string(after) != `{"hit":2}` || hits != 2 {
		t.Fatalf("expected refreshed cache, got %q after %d hits", after, hits)
	}
}

func TestSkillRankingsServeStaleWhileRefreshing(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		n := hits
		mu.Unlock()
		if n > 1 {
			<-release
		}
		fmt.Fprintf(w, `{"hit":%d}`, n)
	}))
	defer server.Close()
	defer close(release)

	b, err := NewBitjitaCache(server.URL, t.TempDir(), cache.New(time.Millisecond))
	if err != nil {
		t.Fatalf("NewBitjitaCache: %v", err)
	}
	if got := BitjitaCachePolicy("/players/7/skill-rankings", ""); got != "stale" {
		t.Fatalf("skill rankings policy = %q, want stale", got)
	}

	first, _, status, _ := b.GetAPI("/players/7/skill-rankings", "")
	if status != "MISS" || string(first) != `{"hit":1}` {
		t.Fatalf("first load = %q status %q", first, status)
	}
	time.Sleep(5 * time.Millisecond)

	// The upstream now hangs: the stored answer must still come back at once.
	done := make(chan struct{})
	go func() {
		defer close(done)
		again, _, status, _ := b.GetAPI("/players/7/skill-rankings", "")
		if status != "STALE" || string(again) != `{"hit":1}` {
			t.Errorf("stale load = %q status %q", again, status)
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stale answer waited on the upstream")
	}
}
