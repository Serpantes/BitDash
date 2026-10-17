package delivery

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bitdash/backend/internal/cache"
	"bitdash/backend/internal/repository"
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

	root := filepath.Join(os.TempDir(), "bitdash-icon-cache-test")
	defer os.RemoveAll(root)

	bitjitaCache, err := repository.NewBitjitaCache(upstream.URL, root, cache.New(time.Minute))
	if err != nil {
		t.Fatalf("NewBitjitaCache: %v", err)
	}

	handler := NewBitjitaIconHandler(bitjitaCache)
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

	upstream.Close()
	res2 := httptest.NewRecorder()
	handler(res2, req)
	if res2.Code != http.StatusOK {
		t.Fatalf("expected cached status 200, got %d", res2.Code)
	}
}
