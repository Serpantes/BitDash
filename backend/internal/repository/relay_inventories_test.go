package repository

import (
	"encoding/json"
	"testing"
)

// A cargo bin row as relay.bitjita.com sends it: cargo in pockets 0 and 5, the rest empty.
const relayCargoBinRow = `[864691128504453383,[[600000,[0,[150000,88,[1,[]],[1,[]]]],false],[600000,[1,[]],false],[600000,[1,[]],false],[600000,[1,[]],false],[600000,[1,[]],false],[600000,[0,[250000,59,[1,[]],[1,[]]]],false]],0,0,864691128504453381,0]`

func TestParseRelayInventory(t *testing.T) {
	row, err := parseRelayInventory(relayCargoBinRow)
	if err != nil {
		t.Fatal(err)
	}
	if row.EntityID != "864691128504453383" || row.OwnerID != "864691128504453381" || row.CargoIndex != 0 || len(row.Pockets) != 6 {
		t.Fatalf("row %+v", row)
	}
	if p := row.Pockets[5]; p.ItemID != "250000" || p.Quantity != "59" || !p.Cargo {
		t.Fatalf("pocket 5 %+v", p)
	}
	if row.Pockets[1].ItemID != "" {
		t.Fatalf("pocket 1 should be empty: %+v", row.Pockets[1])
	}
}

func TestAnnotateTakesRelayPockets(t *testing.T) {
	relay := NewRelayInventories("http://relay.invalid", t.TempDir())
	relay.regions[12] = &relayRegion{region: 12, resub: make(chan struct{}, 1)} // no connection in tests
	relay.apply(12, false, []relayTableUpdate{{TableName: "inventory_state", Updates: []struct {
		Deletes []string `json:"deletes"`
		Inserts []string `json:"inserts"`
	}{{Inserts: []string{relayCargoBinRow}}}}})

	s := NewInventorySlots("")
	s.UseRelay(relay)
	// The housing listing names the building; BitJita's pockets are stale and packed.
	body := []byte(`{"regionId":12,"inventories":[{"entityId":"864691128504453381","inventory":[{"contents":{"item_id":150000,"quantity":80,"item_type":"cargo"}}]}]}`)
	var res struct {
		Inventories []struct {
			CargoPockets int `json:"cargoPockets"`
			Inventory    []struct {
				Slot     int `json:"slot"`
				Contents struct {
					ItemID   int    `json:"item_id"`
					Quantity int    `json:"quantity"`
					ItemType string `json:"item_type"`
				} `json:"contents"`
			} `json:"inventory"`
		} `json:"inventories"`
	}
	if err := json.Unmarshal(s.Annotate("/players/1/housing/2", "", body), &res); err != nil {
		t.Fatal(err)
	}
	inv := res.Inventories[0]
	if inv.CargoPockets != 6 || len(inv.Inventory) != 2 {
		t.Fatalf("inventory %+v", inv)
	}
	if p := inv.Inventory[1]; p.Slot != 5 || p.Contents.ItemID != 250000 || p.Contents.Quantity != 59 || p.Contents.ItemType != "cargo" {
		t.Fatalf("pocket %+v", p)
	}
	if _, watched := relay.watched["864691128504453381"]; !watched {
		t.Fatal("listed storage not watched")
	}
}
