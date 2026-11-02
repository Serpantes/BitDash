package repository

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RelayInventories keeps live copies of BitCraft's inventory_state rows from relay.bitjita.com,
// a read-only replica of the game's SpacetimeDB modules (one per region, "bitcraft-live-<n>").
// Unlike BitJita's REST listings these rows hold every pocket, empty ones included, so a stack's
// pocket is simply its index. Only inventories BitDash has served are watched; the replica
// pushes their changes as they happen.
type RelayInventories struct {
	baseURL string // https://relay.bitjita.com
	dir     string

	mu      sync.Mutex
	token   string
	watched map[string]relayWatch // inventory or owner entity id -> where to look
	rows    map[string]RelayInventory
	regions map[int]*relayRegion
}

type relayWatch struct {
	Region int   `json:"r"`
	Seen   int64 `json:"t"`
}

// RelayInventory is one inventory_state row.
type RelayInventory struct {
	EntityID   string
	OwnerID    string
	CargoIndex int // pockets before this index hold items, the rest cargo
	Pockets    []RelayPocket
}

// RelayPocket is one pocket; ItemID is empty when the pocket is.
type RelayPocket struct {
	ItemID   string
	Cargo    bool
	Quantity string
	Volume   string
	Locked   bool
}

type relayRegion struct {
	region int
	resub  chan struct{}
}

const (
	relayIdentityFile = "relay-identity.json"
	relayWatchFile    = "relay-watch.json"
	// Inventories not served for this long stop being watched.
	relayWatchKeep = 7 * 24 * time.Hour
	relayPingEvery = 30 * time.Second
	relayReadIdle  = 90 * time.Second
)

func NewRelayInventories(baseURL, dir string) *RelayInventories {
	r := &RelayInventories{
		baseURL: strings.TrimRight(baseURL, "/"),
		dir:     dir,
		watched: map[string]relayWatch{},
		rows:    map[string]RelayInventory{},
		regions: map[int]*relayRegion{},
	}
	if data, err := os.ReadFile(filepath.Join(dir, relayIdentityFile)); err == nil {
		var id struct{ Token string }
		if json.Unmarshal(data, &id) == nil {
			r.token = id.Token
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, relayWatchFile)); err == nil {
		_ = json.Unmarshal(data, &r.watched)
	}
	return r
}

// Start connects to every region that has watched inventories.
func (r *RelayInventories) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, w := range r.watched {
		r.regionLocked(w.Region)
	}
}

// regionOf reads the region an entity was created in from the top byte of its id. Buildings
// stay there; player and deployable inventories follow their owner, so listings pass a hint.
func regionOf(entityID string) int {
	id, err := strconv.ParseUint(entityID, 10, 64)
	if err != nil {
		return 0
	}
	return int(id >> 56)
}

// Watch asks for live pockets of an inventory (by its own or its owner's entity id).
func (r *RelayInventories) Watch(entityID string, regionHint int) {
	region := regionHint
	if region <= 0 {
		region = regionOf(entityID)
	}
	if region <= 0 || entityID == "" {
		return
	}
	now := time.Now().Unix()
	r.mu.Lock()
	defer r.mu.Unlock()
	prev, known := r.watched[entityID]
	r.watched[entityID] = relayWatch{Region: region, Seen: now}
	if known && prev.Region == region && now-prev.Seen < 3600 {
		return
	}
	r.saveWatchedLocked(now)
	if known && prev.Region != region {
		r.signalLocked(prev.Region)
	}
	r.signalLocked(region)
}

// Lookup returns the live row of an inventory, found by its own entity id or, when the
// owner holds exactly one inventory, by the owner's.
func (r *RelayInventories) Lookup(entityID string) (RelayInventory, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if row, ok := r.rows[entityID]; ok {
		return row, true
	}
	var found RelayInventory
	n := 0
	for _, row := range r.rows {
		if row.OwnerID == entityID {
			found = row
			n++
		}
	}
	return found, n == 1
}

func (r *RelayInventories) signalLocked(region int) {
	reg := r.regionLocked(region)
	select {
	case reg.resub <- struct{}{}:
	default:
	}
}

func (r *RelayInventories) regionLocked(region int) *relayRegion {
	if reg, ok := r.regions[region]; ok {
		return reg
	}
	reg := &relayRegion{region: region, resub: make(chan struct{}, 1)}
	r.regions[region] = reg
	go r.runRegion(reg)
	return reg
}

func (r *RelayInventories) saveWatchedLocked(now int64) {
	cutoff := now - int64(relayWatchKeep/time.Second)
	for id, w := range r.watched {
		if w.Seen < cutoff {
			delete(r.watched, id)
		}
	}
	data, err := json.Marshal(r.watched)
	if err != nil {
		return
	}
	path := filepath.Join(r.dir, relayWatchFile)
	if os.WriteFile(path+".tmp", data, 0o644) == nil {
		_ = os.Rename(path+".tmp", path)
	}
}

func (r *RelayInventories) queriesFor(region int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for id, w := range r.watched {
		if w.Region == region {
			out = append(out, fmt.Sprintf("SELECT * FROM inventory_state WHERE entity_id = %s OR owner_entity_id = %s", id, id))
		}
	}
	return out
}

func (r *RelayInventories) runRegion(reg *relayRegion) {
	backoff := 5 * time.Second
	for {
		started := time.Now()
		err := r.session(reg)
		if time.Since(started) > time.Minute {
			backoff = 5 * time.Second
		}
		log.Printf("relay region %d: %v; reconnecting in %s", reg.region, err, backoff)
		time.Sleep(backoff)
		if backoff < 5*time.Minute {
			backoff *= 2
		}
	}
}

func (r *RelayInventories) identity(renew bool) (string, error) {
	r.mu.Lock()
	token := r.token
	r.mu.Unlock()
	if token != "" && !renew {
		return token, nil
	}
	resp, err := http.Post(r.baseURL+"/v1/identity", "application/json", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var id struct{ Token string }
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &id) != nil || id.Token == "" {
		return "", fmt.Errorf("relay identity: %s", resp.Status)
	}
	path := filepath.Join(r.dir, relayIdentityFile)
	if os.WriteFile(path+".tmp", body, 0o600) == nil {
		_ = os.Rename(path+".tmp", path)
	}
	r.mu.Lock()
	r.token = id.Token
	r.mu.Unlock()
	return id.Token, nil
}

func (r *RelayInventories) session(reg *relayRegion) error {
	token, err := r.identity(false)
	if err != nil {
		return err
	}
	wsURL := strings.Replace(r.baseURL, "http", "ws", 1) + fmt.Sprintf("/v1/database/bitcraft-live-%d/subscribe", reg.region)
	dialer := websocket.Dialer{Subprotocols: []string{"v1.json.spacetimedb"}, HandshakeTimeout: 20 * time.Second}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	conn, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			_, _ = r.identity(true)
		}
		return err
	}
	defer conn.Close()

	readErr := make(chan error, 1)
	go func() {
		for {
			conn.SetReadDeadline(time.Now().Add(relayReadIdle)) //nolint:errcheck
			_, msg, err := conn.ReadMessage()
			if err != nil {
				readErr <- err
				return
			}
			r.handleMessage(reg.region, msg)
		}
	}()
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(relayReadIdle)) })

	requestID := 0
	subscribe := func() error {
		queries := r.queriesFor(reg.region)
		if len(queries) == 0 {
			return nil
		}
		requestID++
		return conn.WriteJSON(map[string]any{"Subscribe": map[string]any{"query_strings": queries, "request_id": requestID}})
	}
	if err := subscribe(); err != nil {
		return err
	}
	ping := time.NewTicker(relayPingEvery)
	defer ping.Stop()
	for {
		select {
		case err := <-readErr:
			return err
		case <-reg.resub:
			// Let a burst of new inventories (a whole claim listing) settle into one subscription.
			time.Sleep(time.Second)
			if err := subscribe(); err != nil {
				return err
			}
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
				return err
			}
		}
	}
}

type relayTableUpdate struct {
	TableName string `json:"table_name"`
	Updates   []struct {
		Deletes []string `json:"deletes"`
		Inserts []string `json:"inserts"`
	} `json:"updates"`
}

func (r *RelayInventories) handleMessage(region int, msg []byte) {
	var envelope struct {
		InitialSubscription *struct {
			DatabaseUpdate struct {
				Tables []relayTableUpdate `json:"tables"`
			} `json:"database_update"`
		}
		TransactionUpdate *struct {
			Status struct {
				Committed *struct {
					Tables []relayTableUpdate `json:"tables"`
				}
			} `json:"status"`
		}
	}
	if json.Unmarshal(msg, &envelope) != nil {
		return
	}
	switch {
	case envelope.InitialSubscription != nil:
		r.apply(region, true, envelope.InitialSubscription.DatabaseUpdate.Tables)
	case envelope.TransactionUpdate != nil && envelope.TransactionUpdate.Status.Committed != nil:
		r.apply(region, false, envelope.TransactionUpdate.Status.Committed.Tables)
	}
}

// apply stores rows from the replica. An initial subscription answer holds the region's whole
// watched set, so it replaces what the region had.
func (r *RelayInventories) apply(region int, replace bool, tables []relayTableUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if replace {
		for id, row := range r.rows {
			if r.watched[id].Region == region || r.watched[row.OwnerID].Region == region {
				delete(r.rows, id)
			}
		}
	}
	for _, table := range tables {
		if table.TableName != "inventory_state" {
			continue
		}
		for _, u := range table.Updates {
			for _, raw := range u.Deletes {
				if row, err := parseRelayInventory(raw); err == nil {
					delete(r.rows, row.EntityID)
				}
			}
			for _, raw := range u.Inserts {
				if row, err := parseRelayInventory(raw); err == nil {
					r.rows[row.EntityID] = row
				}
			}
		}
	}
}

// parseRelayInventory reads an inventory_state row in SpacetimeDB's JSON encoding:
// [entity_id, pockets, inventory_index, cargo_index, owner_entity_id, player_owner_entity_id],
// a pocket being [volume, contents, locked] and contents an option ([0, stack] or [1, []])
// of [item_id, quantity, item_type ([0, []] item, [1, []] cargo), durability].
func parseRelayInventory(raw string) (RelayInventory, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()
	var row []json.RawMessage
	if err := dec.Decode(&row); err != nil {
		return RelayInventory{}, err
	}
	if len(row) < 5 {
		return RelayInventory{}, errors.New("short inventory_state row")
	}
	var pockets [][]json.RawMessage
	if err := json.Unmarshal(row[1], &pockets); err != nil {
		return RelayInventory{}, err
	}
	cargoIndex, err := strconv.Atoi(string(row[3]))
	if err != nil {
		return RelayInventory{}, err
	}
	out := RelayInventory{
		EntityID:   string(row[0]),
		OwnerID:    string(row[4]),
		CargoIndex: cargoIndex,
		Pockets:    make([]RelayPocket, len(pockets)),
	}
	for i, p := range pockets {
		if len(p) < 3 {
			return RelayInventory{}, errors.New("short pocket")
		}
		pocket := RelayPocket{Volume: string(p[0]), Locked: string(p[2]) == "true"}
		var contents []json.RawMessage
		if err := json.Unmarshal(p[1], &contents); err != nil || len(contents) < 2 {
			return RelayInventory{}, errors.New("bad pocket contents")
		}
		if string(contents[0]) == "0" {
			var stack []json.RawMessage
			if err := json.Unmarshal(contents[1], &stack); err != nil || len(stack) < 3 {
				return RelayInventory{}, errors.New("bad item stack")
			}
			var itemType []json.RawMessage
			if err := json.Unmarshal(stack[2], &itemType); err != nil || len(itemType) < 1 {
				return RelayInventory{}, errors.New("bad item type")
			}
			pocket.ItemID = string(stack[0])
			pocket.Quantity = string(stack[1])
			pocket.Cargo = string(itemType[0]) == "1"
		}
		out.Pockets[i] = pocket
	}
	return out, nil
}
