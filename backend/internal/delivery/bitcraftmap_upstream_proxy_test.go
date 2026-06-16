package delivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBitcraftMapUpstreamProxyHandler(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/region17/resource/128" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	host := upstream.URL[strings.Index(upstream.URL, "//")+2:]
	bitcraftMapUpstreamHosts[host] = upstream.URL

	req := httptest.NewRequest(http.MethodGet, "/api/map-upstream/"+host+"/region17/resource/128", nil)
	rec := httptest.NewRecorder()
	NewBitcraftMapUpstreamProxyHandler(nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	delete(bitcraftMapUpstreamHosts, host)
}
