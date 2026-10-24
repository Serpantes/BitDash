package repository

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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
