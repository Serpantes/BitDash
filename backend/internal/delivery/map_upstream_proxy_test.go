package delivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bitdash/backend/internal/cache"
)

func TestMapUpstreamProxyHandler(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/manifest.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":true}`))
		case "/v1/l5/0_0.bin.gz":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write([]byte{1, 2, 3})
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	host := upstream.URL[strings.Index(upstream.URL, "//")+2:]
	mapUpstreamHosts[host] = upstream.URL
	defer delete(mapUpstreamHosts, host)
	handler := NewMapUpstreamProxyHandler(cache.New(time.Minute))

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/map-upstream/"+host+"/v1/manifest.json", nil))
		if rec.Code != http.StatusOK || rec.Body.String() != `{"ok":true}` {
			t.Fatalf("manifest: status %d body %s", rec.Code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/map-upstream/"+host+"/v1/l5/0_0.bin.gz", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/octet-stream" || rec.Body.Len() != 3 {
		t.Fatalf("tile: status %d type %q len %d", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/map-upstream/evil.example/x", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected unknown hosts refused, got %d", rec.Code)
	}
}
