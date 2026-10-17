package repository

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const marketCatalogFile = "market-catalog.json"

var marketCatalogExcludedTags = map[string]bool{"Recipe": true, "Coins": true}

type MarketGood struct {
	ID            string `json:"id"`
	ItemType      int    `json:"itemType"`
	Name          string `json:"name"`
	IconAssetName string `json:"iconAssetName,omitempty"`
	Tier          int    `json:"tier"`
	RarityStr     string `json:"rarityStr,omitempty"`
	Tag           string `json:"tag,omitempty"`
	HasSell       bool   `json:"hasSell,omitempty"`
	HasBuy        bool   `json:"hasBuy,omitempty"`
}

type marketCatalogPayload struct {
	BuiltAt int64        `json:"builtAt"`
	Goods   []MarketGood `json:"goods"`
}

type marketListingRow struct {
	ID              flexibleID `json:"id"`
	ItemType        int        `json:"itemType"`
	Name            string     `json:"name"`
	IconAssetName   string     `json:"iconAssetName"`
	Tier            int        `json:"tier"`
	RarityStr       string     `json:"rarityStr"`
	Tag             string     `json:"tag"`
	CompendiumEntry bool       `json:"compendiumEntry"`
	HasOrders       bool       `json:"hasOrders"`
	HasSellOrders   bool       `json:"hasSellOrders"`
	HasBuyOrders    bool       `json:"hasBuyOrders"`
}

type flexibleID string

func (f *flexibleID) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = flexibleID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*f = flexibleID(n.String())
	return nil
}

func isTradableGood(row marketListingRow) bool {
	if row.ID == "" || strings.TrimSpace(row.Name) == "" || marketCatalogExcludedTags[row.Tag] {
		return false
	}
	if row.ItemType == 1 {
		return true
	}
	return row.CompendiumEntry || row.HasOrders
}

func buildMarketCatalog(listing []byte, builtAt time.Time) ([]byte, error) {
	var envelope struct {
		Data struct {
			Items []marketListingRow `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listing, &envelope); err != nil {
		return nil, fmt.Errorf("market listing: %w", err)
	}

	goods := make([]MarketGood, 0, len(envelope.Data.Items))
	for _, row := range envelope.Data.Items {
		if !isTradableGood(row) {
			continue
		}
		itemType := 0
		if row.ItemType == 1 {
			itemType = 1
		}
		goods = append(goods, MarketGood{
			ID:            string(row.ID),
			ItemType:      itemType,
			Name:          row.Name,
			IconAssetName: row.IconAssetName,
			Tier:          row.Tier,
			RarityStr:     row.RarityStr,
			Tag:           row.Tag,
			HasSell:       row.HasSellOrders,
			HasBuy:        row.HasBuyOrders,
		})
	}
	if len(goods) == 0 {
		return nil, errors.New("market listing: no tradable goods")
	}
	sort.Slice(goods, func(i, j int) bool {
		if goods[i].Name != goods[j].Name {
			return goods[i].Name < goods[j].Name
		}
		if goods[i].ItemType != goods[j].ItemType {
			return goods[i].ItemType < goods[j].ItemType
		}
		return goods[i].ID < goods[j].ID
	})
	return json.Marshal(marketCatalogPayload{BuiltAt: builtAt.UnixMilli(), Goods: goods})
}

type marketCatalogSnapshot struct {
	body    []byte
	gzipped []byte
	builtAt time.Time
}

func newMarketCatalogSnapshot(body []byte, builtAt time.Time) (*marketCatalogSnapshot, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(body); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return &marketCatalogSnapshot{body: body, gzipped: buf.Bytes(), builtAt: builtAt}, nil
}

type MarketCatalog struct {
	bitjita *BitjitaCache
	ttl     time.Duration

	mu         sync.Mutex
	snapshot   *marketCatalogSnapshot
	diskTried  bool
	refreshing bool
}

func NewMarketCatalog(bitjita *BitjitaCache, ttl time.Duration) *MarketCatalog {
	return &MarketCatalog{bitjita: bitjita, ttl: ttl}
}

func (c *MarketCatalog) Get() (body, gzipped []byte, err error) {
	c.mu.Lock()
	if c.snapshot == nil && !c.diskTried {
		c.diskTried = true
		c.snapshot = c.loadFromDisk()
	}
	if c.snapshot == nil {
		defer c.mu.Unlock()
		fresh, err := c.build()
		if err != nil {
			return nil, nil, err
		}
		c.snapshot = fresh
		return fresh.body, fresh.gzipped, nil
	}
	snap := c.snapshot
	if time.Since(snap.builtAt) > c.ttl && !c.refreshing {
		c.refreshing = true
		go c.refresh()
	}
	c.mu.Unlock()
	return snap.body, snap.gzipped, nil
}

func (c *MarketCatalog) refresh() {
	fresh, err := c.build()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshing = false
	if err != nil {
		log.Printf("market catalog refresh: %v", err)
		return
	}
	c.snapshot = fresh
}

func (c *MarketCatalog) build() (*marketCatalogSnapshot, error) {
	listing, status, err := c.bitjita.fetchUpstream("/market", "")
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("market listing: upstream status %d", status)
	}
	builtAt := time.Now()
	body, err := buildMarketCatalog(listing, builtAt)
	if err != nil {
		return nil, err
	}
	_ = c.bitjita.apiFiles.Set(marketCatalogFile, body)
	return newMarketCatalogSnapshot(body, builtAt)
}

func (c *MarketCatalog) loadFromDisk() *marketCatalogSnapshot {
	body, ok := c.bitjita.apiFiles.Get(marketCatalogFile)
	if !ok {
		return nil
	}
	var head struct {
		BuiltAt int64 `json:"builtAt"`
	}
	if err := json.Unmarshal(body, &head); err != nil || head.BuiltAt == 0 {
		return nil
	}
	snap, err := newMarketCatalogSnapshot(body, time.UnixMilli(head.BuiltAt))
	if err != nil {
		return nil
	}
	return snap
}
