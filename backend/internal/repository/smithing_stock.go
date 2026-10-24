package repository

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Ore already gathered for smithing: every inventory the player can reach (own inventories and
// banks, houses, storage of claims where they may use inventories) with its ore counted per tier.
// The player picks which of them to track on the dashboard.

type TierStock struct {
	Chunks       int `json:"chunks,omitempty"`
	Packages     int `json:"packages,omitempty"`
	Pieces       int `json:"pieces,omitempty"`
	Concentrates int `json:"concentrates,omitempty"`
	Molten       int `json:"molten,omitempty"`
}

type StockInventory struct {
	ID     string                `json:"id"`
	Name   string                `json:"name"`
	Place  string                `json:"place,omitempty"`
	Source string                `json:"source"`
	Counts map[string]*TierStock `json:"counts,omitempty"`
}

type SmithingStock struct {
	FetchedAt   int64            `json:"fetchedAt"`
	Inventories []StockInventory `json:"inventories"`
}

type smithingStockEntry struct {
	stock      *SmithingStock
	refreshing bool
}

type stockGood struct {
	tier int
	add  func(t *TierStock, n int)
}

type stockPocket struct {
	Contents *struct {
		ItemID    flexibleID      `json:"itemId"`
		ItemIDAlt flexibleID      `json:"item_id"`
		ItemType  json.RawMessage `json:"itemType"`
		TypeName  string          `json:"item_type"`
		Quantity  int             `json:"quantity"`
	} `json:"contents"`
}

func (p stockPocket) key() (string, int) {
	c := p.Contents
	if c == nil || c.Quantity <= 0 {
		return "", 0
	}
	id := c.ItemID
	if id == "" {
		id = c.ItemIDAlt
	}
	kind := "item"
	if c.TypeName == "cargo" || string(c.ItemType) == "1" || string(c.ItemType) == `"cargo"` {
		kind = "cargo"
	}
	return kind + ":" + string(id), c.Quantity
}

func (s *SmithingOre) stockGoods() (map[string]stockGood, error) {
	chains, err := s.Chains()
	if err != nil {
		return nil, err
	}
	goods := map[string]stockGood{}
	for _, c := range chains {
		tier := c.Tier
		set := func(key string, add func(t *TierStock, n int)) {
			if key != "item:" && key != "cargo:" {
				goods[key] = stockGood{tier: tier, add: add}
			}
		}
		set("cargo:"+c.ChunkCargoID, func(t *TierStock, n int) { t.Chunks += n })
		set("cargo:"+c.PackageCargoID, func(t *TierStock, n int) { t.Packages += n })
		set("item:"+c.PieceItemID, func(t *TierStock, n int) { t.Pieces += n })
		set("item:"+c.ConcentrateItemID, func(t *TierStock, n int) { t.Concentrates += n })
		set("item:"+c.MoltenItemID, func(t *TierStock, n int) { t.Molten += n })
	}
	return goods, nil
}

func countStock(pockets []stockPocket, goods map[string]stockGood) map[string]*TierStock {
	var counts map[string]*TierStock
	for _, p := range pockets {
		key, n := p.key()
		good, ok := goods[key]
		if !ok {
			continue
		}
		if counts == nil {
			counts = map[string]*TierStock{}
		}
		tier := strconv.Itoa(good.tier)
		if counts[tier] == nil {
			counts[tier] = &TierStock{}
		}
		good.add(counts[tier], n)
	}
	return counts
}

// Stock returns the cached inventories at once and refreshes them in the background when stale;
// only a player seen for the first time waits on BitJita.
func (s *SmithingOre) Stock(playerID string) (*SmithingStock, error) {
	s.stockMu.Lock()
	entry := s.stocks[playerID]
	if entry == nil {
		entry = &smithingStockEntry{stock: s.loadStockFromDisk(playerID)}
		s.stocks[playerID] = entry
	}
	if entry.stock == nil {
		s.stockMu.Unlock()
		stock, err := s.fetchStock(playerID)
		if err != nil {
			return nil, err
		}
		s.storeStock(playerID, stock)
		return stock, nil
	}
	stock := entry.stock
	if time.Since(time.UnixMilli(stock.FetchedAt)) > s.stockTTL && !entry.refreshing {
		entry.refreshing = true
		go s.refreshStock(playerID)
	}
	s.stockMu.Unlock()
	return stock, nil
}

func (s *SmithingOre) refreshStock(playerID string) {
	stock, err := s.fetchStock(playerID)
	if err != nil {
		log.Printf("smithing stock %s: %v", playerID, err)
		s.stockMu.Lock()
		s.stocks[playerID].refreshing = false
		s.stockMu.Unlock()
		return
	}
	s.storeStock(playerID, stock)
}

func smithingStockFile(playerID string) string {
	return "smithing-stock-" + playerID + ".json"
}

func (s *SmithingOre) storeStock(playerID string, stock *SmithingStock) {
	s.stockMu.Lock()
	s.stocks[playerID] = &smithingStockEntry{stock: stock}
	s.stockMu.Unlock()
	if body, err := json.Marshal(stock); err == nil {
		_ = s.bitjita.apiFiles.Set(smithingStockFile(playerID), body)
	}
}

func (s *SmithingOre) loadStockFromDisk(playerID string) *SmithingStock {
	body, ok := s.bitjita.apiFiles.Get(smithingStockFile(playerID))
	if !ok {
		return nil
	}
	var stock SmithingStock
	if err := json.Unmarshal(body, &stock); err != nil || stock.FetchedAt == 0 {
		return nil
	}
	return &stock
}

func (s *SmithingOre) getJSON(path string, out interface{}) error {
	body, status, _, err := s.bitjita.GetAPI(path, "")
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s: upstream status %d", path, status)
	}
	return json.Unmarshal(body, out)
}

func (s *SmithingOre) fetchStock(playerID string) (*SmithingStock, error) {
	goods, err := s.stockGoods()
	if err != nil {
		return nil, err
	}

	var own struct {
		Inventories []struct {
			EntityID      flexibleID    `json:"entityId"`
			InventoryName string        `json:"inventoryName"`
			ClaimName     string        `json:"claimName"`
			Pockets       []stockPocket `json:"pockets"`
		} `json:"inventories"`
	}
	if err := s.getJSON("/players/"+playerID+"/inventories", &own); err != nil {
		return nil, err
	}
	stock := &SmithingStock{}
	for _, inv := range own.Inventories {
		stock.Inventories = append(stock.Inventories, StockInventory{
			ID:     string(inv.EntityID),
			Name:   inv.InventoryName,
			Place:  inv.ClaimName,
			Source: "player",
			Counts: countStock(inv.Pockets, goods),
		})
	}

	type building struct {
		EntityID         flexibleID    `json:"entityId"`
		BuildingName     string        `json:"buildingName"`
		BuildingNickname string        `json:"buildingNickname"`
		Inventory        []stockPocket `json:"inventory"`
	}
	addBuildings := func(source, place string, buildings []building) {
		for _, b := range buildings {
			name := b.BuildingNickname
			if name == "" {
				name = b.BuildingName
			}
			stock.Inventories = append(stock.Inventories, StockInventory{
				ID:     string(b.EntityID),
				Name:   name,
				Place:  place,
				Source: source,
				Counts: countStock(b.Inventory, goods),
			})
		}
	}

	var houses []struct {
		BuildingEntityID flexibleID `json:"buildingEntityId"`
		BuildingName     string     `json:"buildingName"`
		IsEmpty          bool       `json:"isEmpty"`
	}
	if err := s.getJSON("/players/"+playerID+"/housing", &houses); err != nil {
		log.Printf("smithing stock housing %s: %v", playerID, err)
	}
	var player struct {
		Player struct {
			Claims []struct {
				EntityID          flexibleID `json:"entityId"`
				Name              string     `json:"name"`
				IsOwner           bool       `json:"isOwner"`
				MemberPermissions struct {
					InventoryPermission int `json:"inventoryPermission"`
				} `json:"memberPermissions"`
			} `json:"claims"`
		} `json:"player"`
	}
	if err := s.getJSON("/players/"+playerID, &player); err != nil {
		log.Printf("smithing stock claims %s: %v", playerID, err)
	}

	// Houses and claims are independent requests; a claim storage listing alone can take seconds.
	type result struct {
		source, place string
		buildings     []building
	}
	results := make([]*result, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	fetch := func(source, place, path, field string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var raw map[string]json.RawMessage
			if err := s.getJSON(path, &raw); err != nil {
				log.Printf("smithing stock %s: %v", path, err)
				return
			}
			var buildings []building
			if err := json.Unmarshal(raw[field], &buildings); err != nil {
				return
			}
			mu.Lock()
			results = append(results, &result{source, place, buildings})
			mu.Unlock()
		}()
	}
	for _, h := range houses {
		if !h.IsEmpty {
			fetch("house", h.BuildingName, "/players/"+playerID+"/housing/"+string(h.BuildingEntityID), "inventories")
		}
	}
	for _, c := range player.Player.Claims {
		if c.IsOwner || c.MemberPermissions.InventoryPermission > 0 {
			fetch("claim", c.Name, "/claims/"+string(c.EntityID)+"/inventories", "buildings")
		}
	}
	wg.Wait()
	for _, r := range results {
		addBuildings(r.source, r.place, r.buildings)
	}
	stock.FetchedAt = time.Now().UnixMilli()
	return stock, nil
}
