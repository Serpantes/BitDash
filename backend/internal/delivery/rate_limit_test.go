package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterBurstThenRefill(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewRateLimiter(2, 3)
	l.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("request %d within burst was refused", i)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("request over burst was allowed")
	}
	if !l.Allow("5.6.7.8") {
		t.Fatal("another client was refused")
	}
	now = now.Add(500 * time.Millisecond)
	if !l.Allow("1.2.3.4") {
		t.Fatal("one token should refill after 1/rps")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("only one token should refill")
	}
}

func TestRateLimiterUsesCloudflareClientIP(t *testing.T) {
	l := NewRateLimiter(1, 1)
	h := l.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	send := func(cfIP string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
		req.RemoteAddr = "172.17.0.1:5000"
		req.Header.Set("CF-Connecting-IP", cfIP)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if send("1.1.1.1") != http.StatusOK || send("2.2.2.2") != http.StatusOK {
		t.Fatal("different clients behind one tunnel must not share a bucket")
	}
	if code := send("1.1.1.1"); code != http.StatusTooManyRequests {
		t.Fatalf("second request = %d, want 429", code)
	}
}
