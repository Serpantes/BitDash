package repository

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"bitdash/backend/internal/cache"
)

const marketListingSample = `{"data":{"items":[
	{"id":"47","name":"Blue Plank Road Recipe","itemType":0,"tier":-1,"tag":"Recipe","rarityStr":"Common","compendiumEntry":false,"hasOrders":false},
	{"id":"1220006","name":"Rough Plank Output","itemType":0,"tier":1,"tag":"Plank","compendiumEntry":false,"itemListId":"1020003","hasOrders":false},
	{"id":"1020004","name":"High Quality Rough Plank","itemType":0,"tier":1,"tag":"Plank","rarityStr":"Epic","compendiumEntry":false,"hasOrders":false},
	{"id":"1020003","name":"Rough Plank","iconAssetName":"GeneratedIcons/Items/PlankUntreated","itemType":0,"tier":1,"tag":"Plank","rarityStr":"Common","compendiumEntry":true,"hasOrders":true,"hasSellOrders":true,"hasBuyOrders":true},
	{"id":"1020001","name":"Rough Untreated Plank","itemType":0,"tier":1,"tag":"Untreated Plank","compendiumEntry":true,"hasOrders":false},
	{"id":"900","name":"Settlement Building Recipes","itemType":0,"tier":0,"tag":"Recipe","compendiumEntry":true,"hasOrders":false},
	{"id":"1","name":"Hex Coin","itemType":0,"tier":0,"tag":"Coins","compendiumEntry":true,"hasOrders":false},
	{"id":"777","name":"Odd Traded Thing","itemType":0,"tier":2,"tag":"Misc","compendiumEntry":false,"hasOrders":true,"hasSellOrders":true},
	{"id":160001,"name":"Rough Wood Plank Package","itemType":1,"tier":1,"tag":"Package","compendiumEntry":false,"hasOrders":true,"hasSellOrders":true}
],"categories":[],"metrics":{}}}`

func TestBuildMarketCatalogKeepsOnlyTradableGoods(t *testing.T) {
	body, err := buildMarketCatalog([]byte(marketListingSample), time.UnixMilli(1700000000000))
	if err != nil {
		t.Fatalf("buildMarketCatalog: %v", err)
	}
	var payload marketCatalogPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if payload.BuiltAt != 1700000000000 {
		t.Fatalf("builtAt: got %d", payload.BuiltAt)
	}

	var ids []string
	for _, good := range payload.Goods {
		ids = append(ids, good.ID)
	}
	want := []string{"777", "1020003", "1020001", "160001"}
	if len(ids) != len(want) {
		t.Fatalf("goods: got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("goods: got %v, want %v", ids, want)
		}
	}

	plank := payload.Goods[1]
	if plank.Name != "Rough Plank" || plank.ItemType != 0 || plank.Tier != 1 || plank.Tag != "Plank" ||
		plank.IconAssetName != "GeneratedIcons/Items/PlankUntreated" || !plank.HasSell || !plank.HasBuy {
		t.Fatalf("unexpected item: %+v", plank)
	}
	pkg := payload.Goods[3]
	if pkg.ItemType != 1 || !pkg.HasSell || pkg.HasBuy {
		t.Fatalf("unexpected cargo: %+v", pkg)
	}
}

func TestBuildMarketCatalogRejectsEmptyListing(t *testing.T) {
	if _, err := buildMarketCatalog([]byte(`{"data":{"items":[]}}`), time.Now()); err == nil {
		t.Fatal("expected an error for a listing without goods")
	}
}

func newMarketUpstream(t *testing.T, hits *int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/market" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(hits, 1)
		w.Write([]byte(marketListingSample)) //nolint:errcheck
	}))
	t.Cleanup(server.Close)
	return server
}

func TestMarketCatalogBuildsOnceAndReloadsFromDisk(t *testing.T) {
	var hits int32
	upstream := newMarketUpstream(t, &hits)
	bitjita, err := NewBitjitaCache(upstream.URL, t.TempDir(), cache.New(time.Minute))
	if err != nil {
		t.Fatalf("NewBitjitaCache: %v", err)
	}

	catalog := NewMarketCatalog(bitjita, time.Hour)
	first, gzipped, err := catalog.Get()
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if len(first) == 0 || len(gzipped) == 0 {
		t.Fatal("expected plain and gzip bodies")
	}
	if _, _, err := catalog.Get(); err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("upstream hits: got %d, want 1", got)
	}

	restarted := NewMarketCatalog(bitjita, time.Hour)
	again, _, err := restarted.Get()
	if err != nil {
		t.Fatalf("Get after restart: %v", err)
	}
	if string(again) != string(first) {
		t.Fatal("expected the disk copy to match the built catalog")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("upstream hits after restart: got %d, want 1", got)
	}
}

func TestMarketCatalogServesStaleCopyWhileRefreshing(t *testing.T) {
	var hits int32
	upstream := newMarketUpstream(t, &hits)
	bitjita, err := NewBitjitaCache(upstream.URL, t.TempDir(), cache.New(time.Minute))
	if err != nil {
		t.Fatalf("NewBitjitaCache: %v", err)
	}

	catalog := NewMarketCatalog(bitjita, time.Nanosecond)
	if _, _, err := catalog.Get(); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	time.Sleep(time.Millisecond)
	if body, _, err := catalog.Get(); err != nil || len(body) == 0 {
		t.Fatalf("stale Get: body %d bytes, err %v", len(body), err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&hits) < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("expected a background refresh, upstream hits: %d", atomic.LoadInt32(&hits))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestMarketCatalogReportsUpstreamFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer upstream.Close()
	bitjita, err := NewBitjitaCache(upstream.URL, t.TempDir(), cache.New(time.Minute))
	if err != nil {
		t.Fatalf("NewBitjitaCache: %v", err)
	}
	if _, _, err := NewMarketCatalog(bitjita, time.Hour).Get(); err == nil {
		t.Fatal("expected an error when BitJita is down and nothing is cached")
	}
}
