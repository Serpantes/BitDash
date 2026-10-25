package repository

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Ore already gathered for smithing: every inventory the player can reach (own inventories and
// banks, deployed wagons and caches, houses, storage of claims where they may use inventories)
// with its ore counted per tier. The player picks which of them to track on the dashboard.
//
// BitJita leaves empty inventories out, so a deployed wagon or cache with nothing in it would
// vanish. Deployed storage from the vault is always listed, keyed by its name so the choice
// survives redeploying, and inventories seen before stay listed as missing once they empty.

type TierStock struct {
	Chunks       int `json:"chunks,omitempty"`
	Packages     int `json:"packages,omitempty"`
	Pieces       int `json:"pieces,omitempty"`
	Concentrates int `json:"concentrates,omitempty"`
	Molten       int `json:"molten,omitempty"`
}

type StockInventory struct {
	ID      string                `json:"id"`
	Name    string                `json:"name"`
	Place   string                `json:"place,omitempty"`
	Source  string                `json:"source"`
	Counts  map[string]*TierStock `json:"counts,omitempty"`
	Missing bool                  `json:"missing,omitempty"`
	// Src is the BitJita listing the inventory comes from: "player", "house:<id>" or "claim:<id>".
	Src string `json:"src"`
}

func deployableStockID(name string) string {
	return "deployable:" + name
}

// StockCraft is a craft of the player's that holds ore along the ingot chain: its output counts
// as gathered, and a smithing step still in progress has its remaining effort left to give XP.
type StockCraft struct {
	ID              string `json:"id"`
	Tier            int    `json:"tier"`
	Kind            string `json:"kind"`
	Output          int    `json:"output"`
	Step            string `json:"step,omitempty"`
	RemainingEffort int    `json:"remainingEffort,omitempty"`
	Building        string `json:"building,omitempty"`
	Place           string `json:"place,omitempty"`
	Completed       bool   `json:"completed,omitempty"`
	Passive         bool   `json:"passive,omitempty"`
}

type SmithingStock struct {
	FetchedAt   int64            `json:"fetchedAt"`
	Inventories []StockInventory `json:"inventories"`
	Crafts      []StockCraft     `json:"crafts"`
	CraftsAt    int64            `json:"craftsAt,omitempty"`
	PassiveAt   int64            `json:"passiveAt,omitempty"`
	// SourceAt holds when each listing was last read, so only stale tracked ones are reread.
	SourceAt map[string]int64 `json:"sourceAt,omitempty"`
	// Username names deployables ("<username>'s Wagon"), kept so rereads need no extra request.
	Username string `json:"username,omitempty"`
}

type craftOutput struct {
	tier       int
	kind, step string
}

func (s *SmithingOre) craftOutputs() (map[string]craftOutput, error) {
	chains, err := s.Chains()
	if err != nil {
		return nil, err
	}
	out := map[string]craftOutput{}
	for _, c := range chains {
		out[c.PieceItemID] = craftOutput{tier: c.Tier, kind: "pieces"}
		out[c.ConcentrateItemID] = craftOutput{tier: c.Tier, kind: "concentrates", step: "concentrate"}
		out[c.MoltenItemID] = craftOutput{tier: c.Tier, kind: "molten"}
		out[c.IngotID] = craftOutput{tier: c.Tier, kind: "ingots", step: "forge"}
	}
	return out, nil
}

// stockCrafts reads the active crafts and, when withPassive, the passive ones (smelting molten
// metal runs for hours, so those are read less often); otherwise the previous passive crafts stay.
func (s *SmithingOre) stockCrafts(playerID string, previous []StockCraft, withPassive bool) []StockCraft {
	outputs, err := s.craftOutputs()
	if err != nil {
		return previous
	}
	var active, passive []StockCraft
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		active = s.craftsAt(playerID, "/players/"+playerID+"/crafts", outputs, false)
	}()
	if withPassive {
		wg.Add(1)
		go func() {
			defer wg.Done()
			passive = s.craftsAt(playerID, "/players/"+playerID+"/passive-crafts", outputs, true)
		}()
	} else {
		for _, c := range previous {
			if c.Passive {
				passive = append(passive, c)
			}
		}
	}
	wg.Wait()
	return append(append([]StockCraft{}, active...), passive...)
}

// craftsAt reads one crafts listing and keeps the player's crafts along the ingot chain.
func (s *SmithingOre) craftsAt(playerID, path string, outputs map[string]craftOutput, passive bool) []StockCraft {
	var res struct {
		CraftResults []struct {
			EntityID             flexibleID `json:"entityId"`
			OwnerEntityID        flexibleID `json:"ownerEntityId"`
			Progress             int        `json:"progress"`
			CraftCount           int        `json:"craftCount"`
			TotalActionsRequired int        `json:"totalActionsRequired"`
			Completed            bool       `json:"completed"`
			BuildingName         string     `json:"buildingName"`
			ClaimName            string     `json:"claimName"`
			CraftedItem          []struct {
				ItemID   flexibleID `json:"item_id"`
				Quantity int        `json:"quantity"`
			} `json:"craftedItem"`
		} `json:"craftResults"`
	}
	if err := s.readJSON(path, &res, true); err != nil {
		log.Printf("smithing stock %s: %v", path, err)
		return nil
	}
	var crafts []StockCraft
	for _, r := range res.CraftResults {
		if string(r.OwnerEntityID) != playerID || len(r.CraftedItem) == 0 {
			continue
		}
		out, ok := outputs[string(r.CraftedItem[0].ItemID)]
		if !ok {
			continue
		}
		count := r.CraftCount
		if count <= 0 {
			count = 1
		}
		craft := StockCraft{
			ID:        string(r.EntityID),
			Tier:      out.tier,
			Kind:      out.kind,
			Output:    count * r.CraftedItem[0].Quantity,
			Step:      out.step,
			Building:  r.BuildingName,
			Place:     r.ClaimName,
			Completed: r.Completed,
			Passive:   passive,
		}
		if out.step != "" && !r.Completed && r.TotalActionsRequired > r.Progress {
			craft.RemainingEffort = r.TotalActionsRequired - r.Progress
		}
		crafts = append(crafts, craft)
	}
	return crafts
}

type smithingStockEntry struct {
	stock     *SmithingStock
	listing   bool
	rereading chan struct{}
	// refresh serialises rereads of tracked listings for one player.
	refresh sync.Mutex
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

// Stock answers from the cached listing. The full listing (every reachable inventory, used to
// pick what to track) is rebuilt rarely in the background; the listings holding tracked
// inventories, and the crafts when asked for, are reread right away once they are ten seconds old.
// Only a player seen for the first time waits for the full listing.
func (s *SmithingOre) Stock(playerID string, tracked []string, crafts, relist bool) (*SmithingStock, error) {
	s.stockMu.Lock()
	entry := s.stocks[playerID]
	if entry == nil {
		entry = &smithingStockEntry{stock: s.loadStockFromDisk(playerID)}
		s.stocks[playerID] = entry
	}
	current := entry.stock
	s.stockMu.Unlock()

	if current == nil {
		listing, err := s.fetchListing(playerID, nil)
		if err != nil {
			return nil, err
		}
		s.storeStock(playerID, listing)
		current = listing
	} else {
		age := time.Since(time.UnixMilli(current.FetchedAt))
		if age > s.listTTL || (relist && age > s.stockTTL) {
			s.relistInBackground(playerID, entry)
		}
	}

	// BitJita now and then takes many seconds on a listing. The reread runs in the background,
	// one at a time per player; a request waits for it only briefly, then gets the last snapshot.
	if done := s.refreshTracked(playerID, entry, tracked, crafts); done != nil {
		select {
		case <-done:
		case <-time.After(s.trackedWait):
		}
	}
	s.stockMu.Lock()
	defer s.stockMu.Unlock()
	return entry.stock, nil
}

// refreshTracked starts rereading the stale tracked listings and the crafts, unless that is
// already under way, and returns a channel closed when the running reread ends.
func (s *SmithingOre) refreshTracked(playerID string, entry *smithingStockEntry, tracked []string, crafts bool) <-chan struct{} {
	s.stockMu.Lock()
	defer s.stockMu.Unlock()
	if entry.rereading != nil {
		return entry.rereading
	}
	current := entry.stock
	stale := s.staleTrackedSources(current, tracked)
	craftsStale := crafts && time.Since(time.UnixMilli(current.CraftsAt)) > s.trackedTTL
	if len(stale) == 0 && !craftsStale {
		return nil
	}
	done := make(chan struct{})
	entry.rereading = done
	go func() {
		defer func() {
			s.stockMu.Lock()
			entry.rereading = nil
			s.stockMu.Unlock()
			close(done)
		}()
		// Inventories and crafts are separate BitJita requests; read them side by side.
		var crafted []StockCraft
		var wg sync.WaitGroup
		passiveStale := time.Since(time.UnixMilli(current.PassiveAt)) > s.passiveTTL
		if craftsStale {
			wg.Add(1)
			go func() {
				defer wg.Done()
				crafted = s.stockCrafts(playerID, current.Crafts, passiveStale)
			}()
		}
		next := s.rereadSources(playerID, current, stale)
		wg.Wait()
		entry.refresh.Lock()
		defer entry.refresh.Unlock()
		s.stockMu.Lock()
		// A full relisting may have landed meanwhile; lay the reread listings over it.
		if latest := entry.stock; latest != current {
			next = mergeReread(latest, next, stale)
		}
		s.stockMu.Unlock()
		if craftsStale {
			next.Crafts = crafted
			next.CraftsAt = time.Now().UnixMilli()
			if passiveStale {
				next.PassiveAt = next.CraftsAt
			}
		}
		s.storeStock(playerID, next)
	}()
	return done
}

// mergeReread takes the reread listings from next and everything else from latest.
func mergeReread(latest, next *SmithingStock, sources []string) *SmithingStock {
	reread := map[string]bool{}
	for _, src := range sources {
		reread[src] = true
	}
	merged := *latest
	merged.SourceAt = map[string]int64{}
	for k, v := range latest.SourceAt {
		merged.SourceAt[k] = v
	}
	merged.Inventories = nil
	for _, inv := range next.Inventories {
		if reread[inv.Src] {
			merged.Inventories = append(merged.Inventories, inv)
		}
	}
	for _, inv := range latest.Inventories {
		if !reread[inv.Src] {
			merged.Inventories = append(merged.Inventories, inv)
		}
	}
	for src := range reread {
		if at, ok := next.SourceAt[src]; ok {
			merged.SourceAt[src] = at
		}
	}
	return &merged
}

func (s *SmithingOre) relistInBackground(playerID string, entry *smithingStockEntry) {
	s.stockMu.Lock()
	if entry.listing {
		s.stockMu.Unlock()
		return
	}
	entry.listing = true
	s.stockMu.Unlock()
	go func() {
		defer func() {
			s.stockMu.Lock()
			entry.listing = false
			s.stockMu.Unlock()
		}()
		s.stockMu.Lock()
		prev := entry.stock
		s.stockMu.Unlock()
		listing, err := s.fetchListing(playerID, prev)
		if err != nil {
			log.Printf("smithing stock %s: %v", playerID, err)
			return
		}
		entry.refresh.Lock()
		defer entry.refresh.Unlock()
		s.stockMu.Lock()
		if latest := entry.stock; latest != nil {
			listing.Crafts, listing.CraftsAt = latest.Crafts, latest.CraftsAt
		}
		s.stockMu.Unlock()
		s.storeStock(playerID, listing)
	}()
}

func (s *SmithingOre) staleTrackedSources(stock *SmithingStock, tracked []string) []string {
	want := map[string]bool{}
	for _, id := range tracked {
		want[id] = true
	}
	seen := map[string]bool{}
	var stale []string
	for _, inv := range stock.Inventories {
		if !want[inv.ID] || inv.Src == "" || seen[inv.Src] {
			continue
		}
		seen[inv.Src] = true
		if time.Since(time.UnixMilli(stock.SourceAt[inv.Src])) > s.trackedTTL {
			stale = append(stale, inv.Src)
		}
	}
	return stale
}

// rereadSources returns a copy of the stock with the given listings read again.
func (s *SmithingOre) rereadSources(playerID string, stock *SmithingStock, sources []string) *SmithingStock {
	next := *stock
	next.SourceAt = map[string]int64{}
	for k, v := range stock.SourceAt {
		next.SourceAt[k] = v
	}
	if len(sources) == 0 {
		return &next
	}
	goods, err := s.stockGoods()
	if err != nil {
		return &next
	}
	username := stock.Username
	if username == "" {
		username = s.username(playerID)
		next.Username = username
	}
	fresh := make([][]StockInventory, len(sources))
	ok := make([]bool, len(sources))
	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Add(1)
		go func(i int, src string) {
			defer wg.Done()
			invs, err := s.readSource(playerID, src, placeOf(stock, src), username, goods, true)
			if err != nil {
				log.Printf("smithing stock %s %s: %v", playerID, src, err)
				return
			}
			fresh[i], ok[i] = invs, true
		}(i, src)
	}
	wg.Wait()

	now := time.Now().UnixMilli()
	replaced := map[string]bool{}
	var merged []StockInventory
	for i, src := range sources {
		if !ok[i] {
			continue
		}
		replaced[src] = true
		next.SourceAt[src] = now
		merged = append(merged, fresh[i]...)
	}
	listed := map[string]bool{}
	for _, inv := range merged {
		listed[inv.ID] = true
	}
	for _, inv := range stock.Inventories {
		if !replaced[inv.Src] {
			merged = append(merged, inv)
			continue
		}
		// Emptied inventories vanish from BitJita; keep them listed so they stay untickable.
		if !listed[inv.ID] {
			inv.Counts = nil
			inv.Missing = true
			merged = append(merged, inv)
		}
	}
	next.Inventories = merged
	return &next
}

func placeOf(stock *SmithingStock, src string) string {
	for _, inv := range stock.Inventories {
		if inv.Src == src {
			return inv.Place
		}
	}
	return ""
}

func smithingStockFile(playerID string) string {
	return "smithing-stock-" + playerID + ".json"
}

func (s *SmithingOre) storeStock(playerID string, stock *SmithingStock) {
	s.stockMu.Lock()
	entry := s.stocks[playerID]
	if entry == nil {
		entry = &smithingStockEntry{}
		s.stocks[playerID] = entry
	}
	entry.stock = stock
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
	if err := json.Unmarshal(body, &stock); err != nil || stock.FetchedAt == 0 || stock.SourceAt == nil {
		return nil
	}
	return &stock
}

func (s *SmithingOre) getJSON(path string, out interface{}) error {
	return s.readJSON(path, out, false)
}

// readJSON reads a BitJita path; fresh skips the proxy's memory cache, which tracked inventories
// and crafts need to be up to date within seconds.
func (s *SmithingOre) readJSON(path string, out interface{}, fresh bool) error {
	get := s.bitjita.GetAPI
	if fresh {
		get = s.bitjita.GetAPIFresh
	}
	body, status, _, err := get(path, "")
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s: upstream status %d", path, status)
	}
	return json.Unmarshal(body, out)
}

type playerClaims struct {
	Player struct {
		Username string `json:"username"`
		Claims   []struct {
			EntityID          flexibleID `json:"entityId"`
			Name              string     `json:"name"`
			IsOwner           bool       `json:"isOwner"`
			MemberPermissions struct {
				InventoryPermission int `json:"inventoryPermission"`
			} `json:"memberPermissions"`
		} `json:"claims"`
	} `json:"player"`
}

func (s *SmithingOre) username(playerID string) string {
	var player playerClaims
	if err := s.getJSON("/players/"+playerID, &player); err != nil {
		return ""
	}
	return player.Player.Username
}

type stockBuilding struct {
	EntityID         flexibleID    `json:"entityId"`
	BuildingName     string        `json:"buildingName"`
	BuildingNickname string        `json:"buildingNickname"`
	Inventory        []stockPocket `json:"inventory"`
}

// readSource reads one BitJita listing and counts the ore in each of its inventories.
func (s *SmithingOre) readSource(playerID, src, place, username string, goods map[string]stockGood, fresh bool) ([]StockInventory, error) {
	kind, id, _ := strings.Cut(src, ":")
	switch kind {
	case "player":
		var own struct {
			Inventories []struct {
				EntityID      flexibleID    `json:"entityId"`
				OwnerEntityID flexibleID    `json:"ownerEntityId"`
				InventoryName string        `json:"inventoryName"`
				ClaimName     string        `json:"claimName"`
				Pockets       []stockPocket `json:"pockets"`
			} `json:"inventories"`
		}
		if err := s.readJSON("/players/"+playerID+"/inventories", &own, fresh); err != nil {
			return nil, err
		}
		// Deployed wagons, carts and caches own their inventory and are named after the
		// player: "Serpantes's Wagon (III)".
		deployedPrefix := username + "'s "
		var out []StockInventory
		for _, inv := range own.Inventories {
			entry := StockInventory{
				ID:     string(inv.EntityID),
				Name:   inv.InventoryName,
				Place:  inv.ClaimName,
				Source: "player",
				Src:    "player",
				Counts: countStock(inv.Pockets, goods),
			}
			if username != "" && inv.OwnerEntityID == inv.EntityID && strings.HasPrefix(inv.InventoryName, deployedPrefix) {
				entry.Name = strings.TrimPrefix(inv.InventoryName, deployedPrefix)
				entry.ID = deployableStockID(entry.Name)
				entry.Source = "deployable"
			}
			out = append(out, entry)
		}
		return out, nil
	case "house", "claim":
		path, field := "/players/"+playerID+"/housing/"+id, "inventories"
		if kind == "claim" {
			path, field = "/claims/"+id+"/inventories", "buildings"
		}
		var raw map[string]json.RawMessage
		if err := s.readJSON(path, &raw, fresh); err != nil {
			return nil, err
		}
		var buildings []stockBuilding
		if err := json.Unmarshal(raw[field], &buildings); err != nil {
			return nil, err
		}
		out := make([]StockInventory, 0, len(buildings))
		for _, b := range buildings {
			name := b.BuildingNickname
			if name == "" {
				name = b.BuildingName
			}
			out = append(out, StockInventory{
				ID:     string(b.EntityID),
				Name:   name,
				Place:  place,
				Source: kind,
				Src:    src,
				Counts: countStock(b.Inventory, goods),
			})
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown source %q", src)
}

// fetchListing reads every inventory the player can reach.
func (s *SmithingOre) fetchListing(playerID string, prev *SmithingStock) (*SmithingStock, error) {
	goods, err := s.stockGoods()
	if err != nil {
		return nil, err
	}
	var player playerClaims
	if err := s.getJSON("/players/"+playerID, &player); err != nil {
		log.Printf("smithing stock player %s: %v", playerID, err)
	}
	username := player.Player.Username

	type source struct{ src, place string }
	sources := []source{{src: "player"}}
	var houses []struct {
		BuildingEntityID flexibleID `json:"buildingEntityId"`
		BuildingName     string     `json:"buildingName"`
		IsEmpty          bool       `json:"isEmpty"`
	}
	if err := s.getJSON("/players/"+playerID+"/housing", &houses); err != nil {
		log.Printf("smithing stock housing %s: %v", playerID, err)
	}
	for _, h := range houses {
		if !h.IsEmpty {
			sources = append(sources, source{"house:" + string(h.BuildingEntityID), h.BuildingName})
		}
	}
	for _, c := range player.Player.Claims {
		if c.IsOwner || c.MemberPermissions.InventoryPermission > 0 {
			sources = append(sources, source{"claim:" + string(c.EntityID), c.Name})
		}
	}

	// A claim storage listing alone can take seconds, so listings are read side by side.
	results := make([][]StockInventory, len(sources))
	errs := make([]error, len(sources))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, src := range sources {
		wg.Add(1)
		go func(i int, src source) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i], errs[i] = s.readSource(playerID, src.src, src.place, username, goods, false)
		}(i, src)
	}
	wg.Wait()
	if errs[0] != nil {
		return nil, errs[0]
	}

	now := time.Now().UnixMilli()
	stock := &SmithingStock{SourceAt: map[string]int64{}, Username: username}
	for i, src := range sources {
		if errs[i] != nil {
			log.Printf("smithing stock %s %s: %v", playerID, src.src, errs[i])
			continue
		}
		stock.SourceAt[src.src] = now
		stock.Inventories = append(stock.Inventories, results[i]...)
	}
	stock.Inventories = append(stock.Inventories, s.deployedStoragePlaceholders(playerID, stock.Inventories)...)
	stock.Inventories = append(stock.Inventories, missingSince(prev, stock.Inventories)...)
	stock.FetchedAt = now
	return stock, nil
}

// deployedStoragePlaceholders lists deployed storage from the vault that BitJita did not return
// because it is empty.
func (s *SmithingOre) deployedStoragePlaceholders(playerID string, listed []StockInventory) []StockInventory {
	var vault struct {
		Collectibles []struct {
			ID        flexibleID `json:"id"`
			Activated bool       `json:"activated"`
		} `json:"collectibles"`
	}
	if err := s.getJSON("/players/"+playerID+"/vault", &vault); err != nil {
		log.Printf("smithing stock vault %s: %v", playerID, err)
		return nil
	}
	var catalog struct {
		Deployables []struct {
			Name                    string     `json:"name"`
			DeployFromCollectibleID flexibleID `json:"deployFromCollectibleId"`
			Storage                 int        `json:"storage"`
			Stockpile               int        `json:"stockpile"`
		} `json:"deployables"`
	}
	if err := s.getJSON("/deployables", &catalog); err != nil {
		log.Printf("smithing stock deployables: %v", err)
		return nil
	}
	storageByCollectible := map[string]string{}
	for _, d := range catalog.Deployables {
		if d.Storage > 0 || d.Stockpile > 0 {
			storageByCollectible[string(d.DeployFromCollectibleID)] = d.Name
		}
	}
	seen := map[string]bool{}
	for _, inv := range listed {
		seen[inv.ID] = true
	}
	var out []StockInventory
	for _, c := range vault.Collectibles {
		name, ok := storageByCollectible[string(c.ID)]
		if !c.Activated || !ok || seen[deployableStockID(name)] {
			continue
		}
		seen[deployableStockID(name)] = true
		out = append(out, StockInventory{ID: deployableStockID(name), Name: name, Source: "deployable", Src: "player", Missing: true})
	}
	return out
}

// missingSince keeps inventories from the previous listing that BitJita no longer returns,
// so a tracked inventory that emptied can still be seen and unticked.
func missingSince(prev *SmithingStock, listed []StockInventory) []StockInventory {
	if prev == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, inv := range listed {
		seen[inv.ID] = true
	}
	var out []StockInventory
	for _, inv := range prev.Inventories {
		if seen[inv.ID] || inv.Source == "deployable" {
			continue
		}
		inv.Counts = nil
		inv.Missing = true
		out = append(out, inv)
	}
	return out
}
