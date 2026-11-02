package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// DashboardStore keeps the pinned dashboard blocks on disk so they survive
// browser storage being cleared or the dashboard being opened from another
// address or browser. Every save is versioned (rev) so a stale tab cannot
// silently overwrite newer blocks, and older versions are kept as backups.
type DashboardStore struct {
	path       string
	backupDir  string
	initial    json.RawMessage
	keep       int
	backupGap  time.Duration
	mu         sync.Mutex
	lastBackup time.Time
}

type DashboardState struct {
	Rev       int64           `json:"rev"`
	UpdatedAt int64           `json:"updatedAt"`
	Blocks    json.RawMessage `json:"blocks"`
}

var ErrDashboardConflict = errors.New("dashboard was changed elsewhere")

func NewDashboardStore(path string) (*DashboardStore, error) {
	s := newDashboardStore(path)
	if _, err := s.Load(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return s, nil
}

// newDashboardStore creates no files: directories appear on the first save,
// so visitors who only look leave nothing on disk.
func newDashboardStore(path string) *DashboardStore {
	return &DashboardStore{
		path:      path,
		backupDir: filepath.Join(filepath.Dir(path), "dashboard-backups"),
		initial:   json.RawMessage("[]"),
		keep:      200,
		backupGap: 10 * time.Minute,
	}
}

// Load returns the saved state, or rev 0 with the initial blocks when nothing was saved yet.
func (s *DashboardStore) Load() (DashboardState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *DashboardStore) load() (DashboardState, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return DashboardState{Blocks: s.initial}, nil
	}
	if err != nil {
		return DashboardState{}, err
	}
	var state DashboardState
	if err := json.Unmarshal(raw, &state); err != nil {
		return DashboardState{}, err
	}
	if len(state.Blocks) == 0 {
		state.Blocks = json.RawMessage("[]")
	}
	return state, nil
}

// Save replaces the blocks when baseRev matches the stored revision (or force
// is set). On a mismatch it returns the current state with ErrDashboardConflict.
func (s *DashboardStore) Save(baseRev int64, blocks json.RawMessage, force bool) (DashboardState, error) {
	var list []json.RawMessage
	if err := json.Unmarshal(blocks, &list); err != nil {
		return DashboardState{}, fmt.Errorf("blocks must be a JSON array: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.load()
	if err != nil {
		return DashboardState{}, err
	}
	if !force && baseRev != current.Rev {
		return current, ErrDashboardConflict
	}
	if current.Rev > 0 {
		s.backup(current, len(list))
	}

	next := DashboardState{Rev: current.Rev + 1, UpdatedAt: time.Now().UnixMilli(), Blocks: blocks}
	payload, err := json.Marshal(next)
	if err != nil {
		return DashboardState{}, err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return DashboardState{}, err
	}
	if err := writeFileAtomic(s.path, payload); err != nil {
		return DashboardState{}, err
	}
	return next, nil
}

// backup copies the current state aside at most every backupGap, and always
// when the new save has fewer blocks, so a removed block can be restored.
func (s *DashboardStore) backup(current DashboardState, nextCount int) {
	var prev []json.RawMessage
	_ = json.Unmarshal(current.Blocks, &prev)
	if nextCount >= len(prev) && time.Since(s.lastBackup) < s.backupGap {
		return
	}
	payload, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return
	}
	name := fmt.Sprintf("dashboard-%s-rev%d.json", time.Now().UTC().Format("20060102T150405"), current.Rev)
	if err := writeFileAtomic(filepath.Join(s.backupDir, name), payload); err != nil {
		return
	}
	s.lastBackup = time.Now()
	s.pruneBackups()
}

func (s *DashboardStore) pruneBackups() {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "dashboard-") && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	if len(names) <= s.keep {
		return
	}
	sort.Strings(names)
	for _, name := range names[:len(names)-s.keep] {
		os.Remove(filepath.Join(s.backupDir, name)) //nolint:errcheck
	}
}

// DashboardStores keeps a separate DashboardStore for every visitor under
// dir/<visitorID>/, so visitors never see or overwrite each other's blocks.
type DashboardStores struct {
	dir      string
	template json.RawMessage
	mu       sync.Mutex
	stores   map[string]*DashboardStore
}

var (
	ErrBadVisitorID  = errors.New("bad visitor id")
	visitorIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// ValidVisitorID reports whether id is safe to use as a directory name.
func ValidVisitorID(id string) bool {
	return visitorIDPattern.MatchString(id)
}

// NewDashboardStores serves dashboards from dir. A visitor who saved nothing
// yet gets the blocks saved in templateFile, or none when it is empty.
func NewDashboardStores(dir, templateFile string) (*DashboardStores, error) {
	s := &DashboardStores{dir: dir, template: json.RawMessage("[]"), stores: map[string]*DashboardStore{}}
	if templateFile != "" {
		t, err := NewDashboardStore(templateFile)
		if err != nil {
			return nil, err
		}
		state, err := t.Load()
		if err != nil {
			return nil, err
		}
		if state.Rev == 0 {
			return nil, fmt.Errorf("template %s has no saved dashboard", templateFile)
		}
		s.template = state.Blocks
	}
	return s, nil
}

// For returns the store of one visitor.
func (s *DashboardStores) For(visitorID string) (*DashboardStore, error) {
	if !ValidVisitorID(visitorID) {
		return nil, ErrBadVisitorID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	store, ok := s.stores[visitorID]
	if !ok {
		store = newDashboardStore(filepath.Join(s.dir, visitorID, "dashboard.json"))
		store.initial = s.template
		s.stores[visitorID] = store
	}
	return store, nil
}

func writeFileAtomic(path string, payload []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
