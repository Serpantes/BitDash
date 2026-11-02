package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type XPHistory struct {
	dir     string
	wsURL   string
	bitjita *BitjitaCache

	mu      sync.Mutex
	tracked map[string]*xpTrackedPlayer
	players map[string]*xpPlayerState

	connMu sync.Mutex
	conn   *websocket.Conn
}

type XPSample struct {
	T int64            `json:"t"`
	S map[string]int64 `json:"s"`
}

type XPHistoryResponse struct {
	PlayerID     string     `json:"playerId"`
	TrackedSince int64      `json:"trackedSince"`
	Now          int64      `json:"now"`
	Live         bool       `json:"live"`
	Samples      []XPSample `json:"samples"`
}

type xpTrackedPlayer struct {
	Since       int64 `json:"since"`
	LastRequest int64 `json:"lastRequest"`
}

type xpPlayerState struct {
	loaded    bool
	samples   []XPSample
	written   map[string]int64
	latest    map[string]int64
	latestAt  int64
	lastWrite int64
}

const (
	xpFlushEvery      = 15 * time.Second
	xpMinSampleGap    = 60
	xpPollEvery       = 10 * time.Minute
	xpUntrackAfter    = 30 * 24 * time.Hour
	xpTrackedFile     = "tracked.json"
	xpWSPingEvery     = 30 * time.Second
	xpWSReadTimeout   = 90 * time.Second
	xpReconnectMaxGap = 60 * time.Second
)

func NewXPHistory(dir, wsURL string, bitjita *BitjitaCache) (*XPHistory, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	h := &XPHistory{
		dir:     dir,
		wsURL:   wsURL,
		bitjita: bitjita,
		tracked: map[string]*xpTrackedPlayer{},
		players: map[string]*xpPlayerState{},
	}
	if data, err := os.ReadFile(filepath.Join(dir, xpTrackedFile)); err == nil {
		if err := json.Unmarshal(data, &h.tracked); err != nil {
			log.Printf("xp-history: bad %s: %v", xpTrackedFile, err)
		}
	}
	return h, nil
}

func (h *XPHistory) Start(ctx context.Context) {
	go h.pollAll()
	go h.runLive(ctx)
	go h.every(ctx, xpFlushEvery, h.flush)
	go h.every(ctx, xpPollEvery, h.pollAll)
}

// Flush saves the samples still held in memory; call it on shutdown.
func (h *XPHistory) Flush() {
	h.flush()
}

func (h *XPHistory) every(ctx context.Context, d time.Duration, fn func()) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn()
		}
	}
}

func (h *XPHistory) Track(playerID string) {
	now := time.Now().Unix()
	h.mu.Lock()
	tp, exists := h.tracked[playerID]
	if !exists {
		tp = &xpTrackedPlayer{Since: now}
		h.tracked[playerID] = tp
	}
	saveNeeded := !exists || now-tp.LastRequest > 3600
	tp.LastRequest = now
	if saveNeeded {
		h.saveTrackedLocked()
	}
	h.mu.Unlock()

	if !exists {
		h.subscribe([]string{playerID})
		go h.poll(playerID)
	}
}

func (h *XPHistory) History(playerID string, since int64) XPHistoryResponse {
	h.mu.Lock()
	defer h.mu.Unlock()
	st := h.stateLocked(playerID)
	resp := XPHistoryResponse{PlayerID: playerID, Now: time.Now().Unix(), Samples: []XPSample{}}
	if tp := h.tracked[playerID]; tp != nil {
		resp.TrackedSince = tp.Since
	}
	if len(st.samples) > 0 && (resp.TrackedSince == 0 || st.samples[0].T < resp.TrackedSince) {
		resp.TrackedSince = st.samples[0].T
	}
	i := sort.Search(len(st.samples), func(i int) bool { return st.samples[i].T > since })
	resp.Samples = append(resp.Samples, st.samples[i:]...)
	if pending := changedSkills(st.written, st.latest); len(pending) > 0 && st.latestAt > since {
		resp.Samples = append(resp.Samples, XPSample{T: st.latestAt, S: pending})
	}
	h.connMu.Lock()
	resp.Live = h.conn != nil
	h.connMu.Unlock()
	return resp
}

func (h *XPHistory) observe(playerID string, at int64, xp map[string]int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.tracked[playerID]; !ok {
		return
	}
	st := h.stateLocked(playerID)
	changed := false
	for skill, v := range xp {
		if v > st.latest[skill] {
			st.latest[skill] = v
			changed = true
		}
	}
	if changed && at > st.latestAt {
		st.latestAt = at
	}
}

func (h *XPHistory) flush() {
	now := time.Now().Unix()
	h.mu.Lock()
	defer h.mu.Unlock()
	for playerID, st := range h.players {
		if !st.loaded || now-st.lastWrite < xpMinSampleGap {
			continue
		}
		changed := changedSkills(st.written, st.latest)
		if len(changed) == 0 {
			continue
		}
		sample := XPSample{T: st.latestAt, S: changed}
		if len(st.samples) > 0 && sample.T <= st.samples[len(st.samples)-1].T {
			sample.T = st.samples[len(st.samples)-1].T + 1
		}
		if err := h.appendSample(playerID, sample); err != nil {
			log.Printf("xp-history: write %s: %v", playerID, err)
			continue
		}
		st.samples = append(st.samples, sample)
		for skill, v := range changed {
			st.written[skill] = v
		}
		st.lastWrite = now
	}
}

func (h *XPHistory) pollAll() {
	h.mu.Lock()
	ids := make([]string, 0, len(h.tracked))
	cutoff := time.Now().Add(-xpUntrackAfter).Unix()
	removed := false
	for id, tp := range h.tracked {
		if tp.LastRequest < cutoff {
			delete(h.tracked, id)
			delete(h.players, id)
			removed = true
			continue
		}
		ids = append(ids, id)
	}
	if removed {
		h.saveTrackedLocked()
	}
	h.mu.Unlock()
	for _, id := range ids {
		h.poll(id)
	}
}

func (h *XPHistory) poll(playerID string) {
	body, status, _, err := h.bitjita.GetAPI("/players/"+playerID+"/skill-rankings", "")
	if err != nil || status != 200 {
		return
	}
	var payload struct {
		Rankings map[string]struct {
			XP int64 `json:"xp"`
		} `json:"rankings"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return
	}
	xp := make(map[string]int64, len(payload.Rankings))
	for skill, r := range payload.Rankings {
		xp[skill] = r.XP
	}
	h.observe(playerID, time.Now().Unix(), xp)
}

func (h *XPHistory) runLive(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		start := time.Now()
		err := h.liveSession(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		log.Printf("xp-history: live socket closed (%v), reconnecting in %s", err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > xpReconnectMaxGap {
			backoff = xpReconnectMaxGap
		}
	}
}

func (h *XPHistory) liveSession(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, h.wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(xpWSReadTimeout)) //nolint:errcheck
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(xpWSReadTimeout))
	})

	h.connMu.Lock()
	h.conn = conn
	h.connMu.Unlock()
	defer func() {
		h.connMu.Lock()
		h.conn = nil
		h.connMu.Unlock()
	}()

	h.mu.Lock()
	ids := make([]string, 0, len(h.tracked))
	for id := range h.tracked {
		ids = append(ids, id)
	}
	h.mu.Unlock()
	h.subscribe(ids)

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(xpWSPingEvery)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				conn.Close()
				return
			case <-ticker.C:
				h.connMu.Lock()
				err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
				h.connMu.Unlock()
				if err != nil {
					conn.Close()
					return
				}
			}
		}
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		conn.SetReadDeadline(time.Now().Add(xpWSReadTimeout)) //nolint:errcheck
		if playerID, at, xp, ok := parseExperienceEvent(payload); ok {
			h.observe(playerID, at, xp)
		}
	}
}

func (h *XPHistory) subscribe(playerIDs []string) {
	if len(playerIDs) == 0 {
		return
	}
	channels := make([]string, len(playerIDs))
	for i, id := range playerIDs {
		channels[i] = "experience_state:" + id
	}
	h.connMu.Lock()
	defer h.connMu.Unlock()
	if h.conn == nil {
		return
	}
	h.conn.WriteJSON(map[string]any{"type": "subscribe", "channels": channels}) //nolint:errcheck
}

func parseExperienceEvent(payload []byte) (playerID string, at int64, xp map[string]int64, ok bool) {
	var msg struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		Data    struct {
			Stacks []struct {
				Quantity int64 `json:"quantity"`
				SkillID  int64 `json:"skill_id"`
			} `json:"experience_stacks"`
			Timestamp int64 `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil || msg.Type != "event" {
		return "", 0, nil, false
	}
	playerID = strings.TrimPrefix(msg.Channel, "experience_state:")
	if playerID == msg.Channel || len(msg.Data.Stacks) == 0 {
		return "", 0, nil, false
	}
	xp = make(map[string]int64, len(msg.Data.Stacks))
	for _, s := range msg.Data.Stacks {
		if s.Quantity > 0 {
			xp[fmt.Sprint(s.SkillID)] = s.Quantity
		}
	}
	at = msg.Data.Timestamp / 1000
	if at <= 0 {
		at = time.Now().Unix()
	}
	return playerID, at, xp, true
}

func changedSkills(written, latest map[string]int64) map[string]int64 {
	var changed map[string]int64
	for skill, v := range latest {
		if v != written[skill] {
			if changed == nil {
				changed = map[string]int64{}
			}
			changed[skill] = v
		}
	}
	return changed
}

func (h *XPHistory) stateLocked(playerID string) *xpPlayerState {
	st := h.players[playerID]
	if st == nil {
		st = &xpPlayerState{written: map[string]int64{}, latest: map[string]int64{}}
		h.players[playerID] = st
	}
	if st.loaded {
		return st
	}
	st.loaded = true
	f, err := os.Open(h.playerFile(playerID))
	if err != nil {
		return st
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var s XPSample
		if json.Unmarshal(scanner.Bytes(), &s) != nil || len(s.S) == 0 {
			continue
		}
		st.samples = append(st.samples, s)
		for skill, v := range s.S {
			st.written[skill] = v
			if v > st.latest[skill] {
				st.latest[skill] = v
			}
		}
		if s.T > st.latestAt {
			st.latestAt = s.T
		}
	}
	return st
}

func (h *XPHistory) appendSample(playerID string, sample XPSample) error {
	line, err := json.Marshal(sample)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(h.playerFile(playerID), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

func (h *XPHistory) playerFile(playerID string) string {
	return filepath.Join(h.dir, sanitizeCacheFileName(playerID)+".jsonl")
}

func (h *XPHistory) saveTrackedLocked() {
	data, err := json.MarshalIndent(h.tracked, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(h.dir, xpTrackedFile)
	if err := os.WriteFile(path+".tmp", data, 0o644); err == nil {
		os.Rename(path+".tmp", path) //nolint:errcheck
	}
}
