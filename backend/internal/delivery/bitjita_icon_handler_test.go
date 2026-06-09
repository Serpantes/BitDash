package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bitdash/backend/internal/cache"
)

func TestBitjitaIconHandlerUsesGeneratedIconsPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/GeneratedIcons/Items/FishAndChips.webp" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/webp")
		w.Write([]byte("webp")) //nolint:errcheck
	}))
	defer upstream.Close()

	handler := NewBitjitaIconHandler(upstream.URL, cache.New(time.Minute))
	req := httptest.NewRequest(http.MethodGet, "/api/bitjita-icon?iconAssetName=Items/FishAndChips", nil)
	res := httptest.NewRecorder()

	handler(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); got != "image/webp" {
		t.Fatalf("expected image/webp content type, got %q", got)
	}
	if got := res.Body.String(); got != "webp" {
		t.Fatalf("expected icon body, got %q", got)
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
