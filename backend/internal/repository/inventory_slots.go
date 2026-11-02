package repository

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

// InventorySlots remembers which pocket every stack sits in. BitJita lists only the filled
// pockets of an inventory, in pocket order but without their index, so taking out a stack in
// the middle would shift every later one left. Each new listing is matched against the previous
// one: stacks that are still there keep their pocket, an emptied pocket stays a gap, and a new
// stack lands in the first free pocket between its neighbours, as a deposit does in game.
// A storage seen for the first time is laid out packed from pocket 0.
type InventorySlots struct {
	mu   sync.Mutex
	path string
	inv  map[string]*slotLayout
}

type slotLayout struct {
	Items []slotEntry `json:"i,omitempty"`
	Cargo []slotEntry `json:"c,omitempty"`
	Seen  int64       `json:"t"`
}

type slotEntry struct {
	Slot int    `json:"s"`
	ID   string `json:"k"`
	Qty  string `json:"q,omitempty"`
}

// Storages not listed for this long are forgotten.
const slotLayoutKeep = 60 * 24 * time.Hour

func NewInventorySlots(path string) *InventorySlots {
	s := &InventorySlots{path: path, inv: map[string]*slotLayout{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s.inv)
	}
	return s
}

// inventoryListing says where a BitJita response keeps its storages and pockets.
type inventoryListing struct {
	list, pockets string
}

func inventoryListingFor(path, query string) (inventoryListing, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	switch {
	case len(parts) == 3 && parts[0] == "players" && parts[2] == "inventories":
		// A name filter drops pockets, so such a listing says nothing about positions.
		if strings.Contains("&"+query, "&q=") {
			return inventoryListing{}, false
		}
		return inventoryListing{"inventories", "pockets"}, true
	case len(parts) == 3 && parts[0] == "claims" && parts[2] == "inventories":
		return inventoryListing{"buildings", "inventory"}, true
	case len(parts) == 4 && parts[0] == "players" && parts[2] == "housing":
		return inventoryListing{"inventories", "inventory"}, true
	}
	return inventoryListing{}, false
}

// Annotate adds "slot" (the pocket index within the item or cargo pockets) to every filled
// pocket of an inventory listing and returns the rewritten body. Other bodies pass unchanged.
func (s *InventorySlots) Annotate(path, query string, body []byte) []byte {
	listing, ok := inventoryListingFor(path, query)
	if !ok {
		return body
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var root map[string]any
	if dec.Decode(&root) != nil {
		return body
	}
	storages, _ := root[listing.list].([]any)
	if len(storages) == 0 {
		return body
	}

	s.mu.Lock()
	now := time.Now().Unix()
	for _, raw := range storages {
		storage, _ := raw.(map[string]any)
		id, _ := storage["entityId"].(string)
		pockets, _ := storage[listing.pockets].([]any)
		if id == "" {
			continue
		}
		var items, cargo []map[string]any
		for _, p := range pockets {
			pocket, _ := p.(map[string]any)
			if pocketID(pocket) == "" {
				continue
			}
			if pocketIsCargo(pocket) {
				cargo = append(cargo, pocket)
			} else {
				items = append(items, pocket)
			}
		}
		prev := s.inv[id]
		if prev == nil {
			prev = &slotLayout{}
		}
		s.inv[id] = &slotLayout{
			Items: placePockets(prev.Items, items),
			Cargo: placePockets(prev.Cargo, cargo),
			Seen:  now,
		}
	}
	s.saveLocked(now)
	s.mu.Unlock()

	out, err := json.Marshal(root)
	if err != nil {
		return body
	}
	return out
}

func pocketID(pocket map[string]any) string {
	contents, _ := pocket["contents"].(map[string]any)
	if contents == nil {
		return ""
	}
	for _, key := range []string{"itemId", "item_id"} {
		if v, ok := contents[key].(json.Number); ok {
			return v.String()
		}
	}
	return ""
}

func pocketQty(pocket map[string]any) string {
	contents, _ := pocket["contents"].(map[string]any)
	if v, ok := contents["quantity"].(json.Number); ok {
		return v.String()
	}
	return ""
}

func pocketIsCargo(pocket map[string]any) bool {
	contents, _ := pocket["contents"].(map[string]any)
	switch t := contents["itemType"].(type) {
	case json.Number:
		return t.String() == "1"
	}
	t, _ := contents["item_type"].(string)
	return t == "cargo"
}

// placePockets writes a slot into each pocket and returns the new layout to remember.
func placePockets(prev []slotEntry, pockets []map[string]any) []slotEntry {
	next := make([]slotEntry, len(pockets))
	for i, p := range pockets {
		next[i] = slotEntry{ID: pocketID(p), Qty: pocketQty(p)}
	}
	matched := matchStacks(prev, next)
	last := -1
	for i := range next {
		slot := last + 1
		if j := matched[i]; j >= 0 && prev[j].Slot > last {
			slot = prev[j].Slot
		}
		next[i].Slot = slot
		pockets[i]["slot"] = slot
		last = slot
	}
	return next
}

// matchStacks pairs new stacks with old ones in order (a longest common subsequence of item
// ids, preferring pairs whose quantity is unchanged) and returns the old index per new stack.
func matchStacks(prev, next []slotEntry) []int {
	n, m := len(prev), len(next)
	score := make([][]int, n+1)
	for i := range score {
		score[i] = make([]int, m+1)
	}
	pair := func(i, j int) int {
		if prev[i].ID != next[j].ID {
			return -1
		}
		if prev[i].Qty == next[j].Qty {
			return 1001
		}
		return 1000
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			best := score[i+1][j]
			if score[i][j+1] > best {
				best = score[i][j+1]
			}
			if p := pair(i, j); p > 0 && p+score[i+1][j+1] > best {
				best = p + score[i+1][j+1]
			}
			score[i][j] = best
		}
	}
	out := make([]int, m)
	for j := range out {
		out[j] = -1
	}
	for i, j := 0, 0; i < n && j < m; {
		if p := pair(i, j); p > 0 && score[i][j] == p+score[i+1][j+1] {
			out[j] = i
			i++
			j++
		} else if score[i+1][j] >= score[i][j+1] {
			i++
		} else {
			j++
		}
	}
	return out
}

func (s *InventorySlots) saveLocked(now int64) {
	cutoff := now - int64(slotLayoutKeep/time.Second)
	for id, layout := range s.inv {
		if layout.Seen < cutoff {
			delete(s.inv, id)
		}
	}
	data, err := json.Marshal(s.inv)
	if err != nil || s.path == "" {
		return
	}
	if os.WriteFile(s.path+".tmp", data, 0o644) == nil {
		_ = os.Rename(s.path+".tmp", s.path)
	}
}
