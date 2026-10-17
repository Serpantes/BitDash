package delivery

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bitdash/backend/internal/cache"
	"bitdash/backend/internal/repository"
)

func newTestMarketCatalog(t *testing.T) *repository.MarketCatalog {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"items":[` + //nolint:errcheck
			`{"id":"1020003","name":"Rough Plank","itemType":0,"tier":1,"tag":"Plank","compendiumEntry":true,"hasOrders":true,"hasSellOrders":true},` +
			`{"id":"47","name":"Blue Plank Road Recipe","itemType":0,"tier":-1,"tag":"Recipe","compendiumEntry":false}` +
			`]}}`))
	}))
	t.Cleanup(upstream.Close)
	bitjita, err := repository.NewBitjitaCache(upstream.URL, t.TempDir(), cache.New(time.Minute))
	if err != nil {
		t.Fatalf("NewBitjitaCache: %v", err)
	}
	return repository.NewMarketCatalog(bitjita, time.Hour)
}

func decodeCatalogGoods(t *testing.T, body io.Reader) []repository.MarketGood {
	t.Helper()
	var payload struct {
		Goods []repository.MarketGood `json:"goods"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	return payload.Goods
}

func TestMarketCatalogHandlerServesGzipWhenAccepted(t *testing.T) {
	handler := NewMarketCatalogHandler(newTestMarketCatalog(t))

	req := httptest.NewRequest(http.MethodGet, "/api/market-catalog", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	res := httptest.NewRecorder()
	handler(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status: got %d: %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding: got %q", got)
	}
	zr, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	goods := decodeCatalogGoods(t, zr)
	if len(goods) != 1 || goods[0].Name != "Rough Plank" {
		t.Fatalf("goods: got %+v", goods)
	}
}

func TestMarketCatalogHandlerServesPlainJSON(t *testing.T) {
	handler := NewMarketCatalogHandler(newTestMarketCatalog(t))

	res := httptest.NewRecorder()
	handler(res, httptest.NewRequest(http.MethodGet, "/api/market-catalog", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status: got %d", res.Code)
	}
	if got := res.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding: got %q, want none", got)
	}
	if goods := decodeCatalogGoods(t, res.Body); len(goods) != 1 {
		t.Fatalf("goods: got %+v", goods)
	}
}

func TestMarketCatalogHandlerRejectsPost(t *testing.T) {
	handler := NewMarketCatalogHandler(newTestMarketCatalog(t))
	res := httptest.NewRecorder()
	handler(res, httptest.NewRequest(http.MethodPost, "/api/market-catalog", nil))
	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d", res.Code)
	}
}
