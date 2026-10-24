package repository

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"bitdash/backend/internal/cache"
)

func TestBuildSmithingSetupPicksStrongestHammerAndStats(t *testing.T) {
	values := make([]string, 55)
	for i := range values {
		values[i] = "0"
	}
	values[statCraftingSpeed] = "1.25"
	values[statSmithingSpeed] = "1.2"
	values[statSmithingPower] = "13"
	values[statExperienceRate] = "114"
	stats := []byte(`{"stats":{"values":[` + strings.Join(values, ",") + `]}}`)
	inv := []byte(`{
		"inventories": [
			{"inventoryName": "Town Bank", "pockets": [{"contents": {"itemId": 3}}]},
			{"inventoryName": "Toolbelt", "pockets": [
				{"contents": {"itemId": 1}},
				{"contents": null},
				{"contents": {"itemId": 2}},
				{"contents": {"itemId": 4}}
			]}
		],
		"items": {
			"1": {"name": "Weak Hammer", "toolPower": 20, "toolLevel": 5, "toolSkillId": 6},
			"2": {"name": "Umbracite Hammer", "toolPower": 41, "toolLevel": 10, "toolSkillId": 6},
			"3": {"name": "Banked Hammer", "toolPower": 99, "toolLevel": 10, "toolSkillId": 6},
			"4": {"name": "Pickaxe", "toolPower": 60, "toolLevel": 10, "toolSkillId": 5}
		}
	}`)

	setup, err := buildSmithingSetup(stats, inv, time.UnixMilli(1000))
	if err != nil {
		t.Fatal(err)
	}
	if setup.Tool == nil || setup.Tool.Name != "Umbracite Hammer" {
		t.Fatalf("tool = %+v, want the strongest toolbelt hammer", setup.Tool)
	}
	if setup.ProgressPerSwing != 54 {
		t.Errorf("progress per swing = %v, want 54", setup.ProgressPerSwing)
	}
	if setup.XPRate != 1.14 {
		t.Errorf("xp rate = %v, want 1.14", setup.XPRate)
	}
	if setup.Speed != 1.5 {
		t.Errorf("speed = %v, want 1.5", setup.Speed)
	}
}

func TestBuildSmithingSetupWithoutHammer(t *testing.T) {
	setup, err := buildSmithingSetup([]byte(`{"stats":{"values":[]}}`), []byte(`{"inventories":[],"items":{}}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if setup.Tool != nil || setup.ProgressPerSwing != 0 || setup.XPRate != 1 {
		t.Errorf("setup = %+v, want no tool, no progress and a neutral rate", setup)
	}
}

func TestParseMarketBookSortsAndDropsBadOrders(t *testing.T) {
	book, err := parseMarketBook([]byte(`{
		"sellOrders": [
			{"priceThreshold": "100", "quantity": "309"},
			{"priceThreshold": "50", "quantity": "3"},
			{"priceThreshold": "0", "quantity": "10"},
			{"priceThreshold": "70", "quantity": "x"}
		],
		"buyOrders": [
			{"priceThreshold": "20", "quantity": "5"},
			{"priceThreshold": "45", "quantity": "1"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(book.Sell) != 2 || book.Sell[0] != [2]float64{50, 3} || book.Sell[1] != [2]float64{100, 309} {
		t.Errorf("sell = %v, want cheapest first without bad orders", book.Sell)
	}
	if len(book.Buy) != 2 || book.Buy[0] != [2]float64{45, 1} {
		t.Errorf("buy = %v, want best bid first", book.Buy)
	}
}

func TestCountStockReadsBothPocketFormats(t *testing.T) {
	goods := map[string]stockGood{
		"cargo:3001":   {tier: 2, add: func(s *TierStock, n int) { s.Chunks += n }},
		"cargo:900":    {tier: 2, add: func(s *TierStock, n int) { s.Packages += n }},
		"item:2040002": {tier: 2, add: func(s *TierStock, n int) { s.Pieces += n }},
		"item:2040003": {tier: 2, add: func(s *TierStock, n int) { s.Concentrates += n }},
	}
	var pockets []stockPocket
	if err := json.Unmarshal([]byte(`[
		{"contents": {"itemId": 3001, "itemType": 1, "quantity": 17}},
		{"contents": {"item_id": 2040002, "item_type": "item", "quantity": 176}},
		{"contents": {"item_id": 900, "item_type": "cargo", "quantity": 2}},
		{"contents": {"itemId": 2040002, "itemType": 0, "quantity": 4}},
		{"contents": {"itemId": 3001, "itemType": 0, "quantity": 99}},
		{"contents": null}
	]`), &pockets); err != nil {
		t.Fatal(err)
	}
	counts := countStock(pockets, goods)
	got := counts["2"]
	if got == nil || got.Chunks != 17 || got.Pieces != 180 || got.Packages != 2 || got.Concentrates != 0 {
		t.Fatalf("counts = %+v, want 17 chunks, 2 packages, 180 pieces (item 3001 is not the cargo chunk)", got)
	}
	if countStock(nil, goods) != nil {
		t.Error("an inventory without ore should have no counts")
	}
}

func TestStockCraftsCountOutputsAndUnfinishedSmithing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/players/7/crafts":
			fmt.Fprint(w, `{"craftResults": [
				{"entityId": "c1", "ownerEntityId": "7", "progress": 1000, "craftCount": 10, "totalActionsRequired": 4100,
				 "buildingName": "Magnificent Smithing Station", "craftedItem": [{"item_id": 30, "quantity": 1}]},
				{"entityId": "c2", "ownerEntityId": "7", "progress": 0, "craftCount": 5, "totalActionsRequired": 2050,
				 "completed": true, "craftedItem": [{"item_id": 20, "quantity": 4}]},
				{"entityId": "c3", "ownerEntityId": "8", "progress": 0, "craftCount": 5, "totalActionsRequired": 100,
				 "craftedItem": [{"item_id": 30, "quantity": 1}]},
				{"entityId": "c4", "ownerEntityId": "7", "progress": 0, "craftCount": 5, "totalActionsRequired": 100,
				 "craftedItem": [{"item_id": 999, "quantity": 1}]}
			]}`)
		case "/api/players/7/passive-crafts":
			fmt.Fprint(w, `{"craftResults": [
				{"entityId": "p1", "ownerEntityId": "7", "craftCount": 2, "craftedItem": [{"item_id": 40, "quantity": 1}]}
			]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	b, err := NewBitjitaCache(server.URL, t.TempDir(), cache.New(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	ore := NewSmithingOre(b, time.Minute, time.Minute)
	ore.chains = []IngotChain{{Tier: 9, IngotID: "50", PieceItemID: "20", ConcentrateItemID: "30", MoltenItemID: "40"}}

	crafts := ore.stockCrafts("7")
	if len(crafts) != 3 {
		t.Fatalf("crafts = %+v, want the player's three chain crafts", crafts)
	}
	byID := map[string]StockCraft{}
	for _, c := range crafts {
		byID[c.ID] = c
	}
	if c := byID["c1"]; c.Kind != "concentrates" || c.Output != 10 || c.Step != "concentrate" || c.RemainingEffort != 3100 {
		t.Errorf("concentrate craft = %+v, want 10 concentrates with 3100 effort left", c)
	}
	if c := byID["c2"]; c.Kind != "pieces" || c.Output != 20 || c.RemainingEffort != 0 {
		t.Errorf("finished piece craft = %+v, want 20 pieces and no effort left", c)
	}
	if c := byID["p1"]; c.Kind != "molten" || c.Output != 2 {
		t.Errorf("passive smelt = %+v, want 2 molten", c)
	}
}

func TestStockRereadsOnlyTrackedSources(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	wagon := `{"entityId": "w1", "ownerEntityId": "w1", "inventoryName": "Ann's Wagon (III)", "pockets": [{"contents": {"itemId": 3001, "itemType": 1, "quantity": 5}}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		switch r.URL.Path {
		case "/api/players/7":
			fmt.Fprint(w, `{"player": {"username": "Ann", "claims": [{"entityId": "c9", "name": "Town", "memberPermissions": {"inventoryPermission": 1}}]}}`)
		case "/api/players/7/inventories":
			mu.Lock()
			body := wagon
			mu.Unlock()
			fmt.Fprintf(w, `{"inventories": [%s]}`, body)
		case "/api/players/7/housing":
			fmt.Fprint(w, `[]`)
		case "/api/claims/c9/inventories":
			fmt.Fprint(w, `{"buildings": [{"entityId": "b1", "buildingName": "Stockpile", "inventory": []}]}`)
		case "/api/players/7/vault", "/api/deployables":
			fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	b, err := NewBitjitaCache(server.URL, t.TempDir(), cache.New(time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	ore := NewSmithingOre(b, time.Minute, time.Minute)
	ore.chains = []IngotChain{{Tier: 2, ChunkCargoID: "3001"}}

	stock, err := ore.Stock("7", nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(stock.Inventories) != 2 || stock.Inventories[0].ID != "deployable:Wagon (III)" || stock.Inventories[0].Counts["2"].Chunks != 5 {
		t.Fatalf("listing = %+v, want the wagon (5 chunks) and the claim stockpile", stock.Inventories)
	}

	// The wagon is emptied and its listing ages past a minute: only that listing is reread.
	mu.Lock()
	wagon = `{"entityId": "x", "inventoryName": "Toolbelt", "pockets": []}`
	hits = map[string]int{}
	mu.Unlock()
	ore.stocks["7"].stock.SourceAt["player"] = time.Now().Add(-2 * time.Minute).UnixMilli()
	stock, err = ore.Stock("7", []string{"deployable:Wagon (III)"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if hits["/api/players/7/inventories"] != 1 || hits["/api/claims/c9/inventories"] != 0 {
		t.Errorf("requests = %v, want only the player's inventories reread", hits)
	}
	var gotWagon *StockInventory
	for i := range stock.Inventories {
		if stock.Inventories[i].ID == "deployable:Wagon (III)" {
			gotWagon = &stock.Inventories[i]
		}
	}
	if gotWagon == nil || !gotWagon.Missing || gotWagon.Counts != nil {
		t.Errorf("wagon = %+v, want it kept as missing with no ore", gotWagon)
	}

	// Fresh enough: nothing is read at all.
	mu.Lock()
	hits = map[string]int{}
	mu.Unlock()
	if _, err := ore.Stock("7", []string{"deployable:Wagon (III)"}, false, false); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("requests = %v, want none while the tracked listing is fresh", hits)
	}
}
