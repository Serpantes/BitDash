package repository

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func playerInventoriesBody(stacks ...string) []byte {
	pockets := make([]string, len(stacks))
	for i, stack := range stacks {
		var id, qty, kind int
		fmt.Sscanf(stack, "%d:%d:%d", &id, &qty, &kind)
		pockets[i] = fmt.Sprintf(`{"locked":false,"volume":6000,"contents":{"itemId":%d,"itemType":%d,"quantity":%d}}`, id, kind, qty)
	}
	return []byte(`{"inventories":[{"entityId":"864691128538634525","cargoIndex":64,"pockets":[` + strings.Join(pockets, ",") + `]}],"items":[],"cargos":[]}`)
}

func annotatedSlots(t *testing.T, body []byte) []int {
	t.Helper()
	var res struct {
		Inventories []struct {
			Pockets []struct {
				Slot *int `json:"slot"`
			} `json:"pockets"`
		} `json:"inventories"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatal(err)
	}
	var out []int
	for _, p := range res.Inventories[0].Pockets {
		if p.Slot == nil {
			t.Fatalf("pocket without slot in %s", body)
		}
		out = append(out, *p.Slot)
	}
	return out
}

func TestInventorySlotsKeepGapsAndFillFirstFree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slots.json")
	s := NewInventorySlots(path)
	const url = "/players/1/inventories"

	steps := []struct {
		stacks []string
		want   []int
	}{
		// First sight: packed. Cargo counts its own pockets.
		{[]string{"10:5:0", "20:1:0", "30:7:0", "40:2:0", "900:3:1"}, []int{0, 1, 2, 3, 0}},
		// Stack 20 taken out: 30 and 40 stay where they were.
		{[]string{"10:5:0", "30:7:0", "40:2:0", "900:3:1"}, []int{0, 2, 3, 0}},
		// Quantity changes do not move a stack; a deposit fills the gap at pocket 1.
		{[]string{"10:9:0", "50:1:0", "30:7:0", "40:2:0", "900:4:1"}, []int{0, 1, 2, 3, 0}},
		// A stack added after the last one goes right after it.
		{[]string{"10:9:0", "50:1:0", "30:7:0", "40:2:0", "60:1:0"}, []int{0, 1, 2, 3, 4}},
	}
	for i, step := range steps {
		got := annotatedSlots(t, s.Annotate(url, "", playerInventoriesBody(step.stacks...)))
		if !reflect.DeepEqual(got, step.want) {
			t.Fatalf("step %d: slots %v, want %v", i, got, step.want)
		}
	}

	// The layout survives a restart.
	again := NewInventorySlots(path)
	got := annotatedSlots(t, again.Annotate(url, "", playerInventoriesBody("10:9:0", "40:2:0")))
	if !reflect.DeepEqual(got, []int{0, 3}) {
		t.Fatalf("after reload: slots %v, want [0 3]", got)
	}
}

func TestInventorySlotsPrefersSameQuantity(t *testing.T) {
	s := NewInventorySlots("")
	const url = "/players/1/inventories"
	s.Annotate(url, "", playerInventoriesBody("7:1000:0", "7:1000:0", "7:400:0"))
	got := annotatedSlots(t, s.Annotate(url, "", playerInventoriesBody("7:1000:0", "7:400:0")))
	if !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("slots %v, want [0 2]", got)
	}
}

func TestInventorySlotsClaimListingAndFilteredSearch(t *testing.T) {
	s := NewInventorySlots("")
	claim := []byte(`{"buildings":[{"entityId":"5","inventory":[{"contents":{"item_id":1,"quantity":2,"item_type":"item"}},{"contents":{"item_id":3,"quantity":4,"item_type":"cargo"}}]}]}`)
	out := string(s.Annotate("/claims/9/inventories", "", claim))
	if strings.Count(out, `"slot":0`) != 2 {
		t.Fatalf("claim listing not annotated: %s", out)
	}
	body := playerInventoriesBody("1:1:0")
	if got := s.Annotate("/players/1/inventories", "q=ore", body); string(got) != string(body) {
		t.Fatalf("filtered listing changed: %s", got)
	}
	if got := s.Annotate("/players/1/market", "", body); string(got) != string(body) {
		t.Fatalf("unrelated path changed: %s", got)
	}
}

func TestBuildStorageCapacity(t *testing.T) {
	buildings := []byte(`[{"name":"Town Bank","functions":[{"storage_slots":64,"cargo_slots":16}]},
		{"name":"Exquisite Barter Stall","functions":[{"storage_slots":60},{"cargo_slots":18}]},
		{"name":"Fence","functions":[{"storage_slots":0}]}]`)
	deployables := []byte(`{"deployables":[{"name":"Personal Cache (III)","storage":64,"stockpile":24},{"name":"Bird (I)","storage":6,"stockpile":0}]}`)
	body, err := buildStorageCapacity(buildings, deployables, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	var got storageCapacityPayload
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{"Town Bank": {64, 16}, "Exquisite Barter Stall": {60, 18}, "Personal Cache (III)": {64, 24}, "Bird (I)": {6, 0}}
	if !reflect.DeepEqual(got.Storages, want) {
		t.Fatalf("storages %v, want %v", got.Storages, want)
	}
}
