package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseExperienceEvent(t *testing.T) {
	payload := []byte(`{"type":"event","channel":"experience_state:42","data":{"entity_id":"42","experience_stacks":[{"quantity":0,"skill_id":1},{"quantity":118117018,"skill_id":6}],"timestamp":1790189771524}}`)
	id, at, xp, ok := parseExperienceEvent(payload)
	if !ok || id != "42" || at != 1790189771 {
		t.Fatalf("got id=%q at=%d ok=%v", id, at, ok)
	}
	if len(xp) != 1 || xp["6"] != 118117018 {
		t.Fatalf("unexpected xp: %#v", xp)
	}
	if _, _, _, ok := parseExperienceEvent([]byte(`{"type":"subscribed","channels":["experience_state:42"]}`)); ok {
		t.Fatal("subscribe ack must not parse as an event")
	}
}

func TestXPHistoryRecordsChangedSkillsAndReloads(t *testing.T) {
	dir := t.TempDir()
	h, err := NewXPHistory(dir, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	h.tracked["42"] = &xpTrackedPlayer{Since: 100}

	h.observe("42", 100, map[string]int64{"6": 1000, "5": 500})
	h.flush()
	h.observe("42", 200, map[string]int64{"6": 900})
	h.observe("42", 210, map[string]int64{"6": 1300, "5": 500})
	h.players["42"].lastWrite = 0
	h.flush()

	resp := h.History("42", 0)
	if len(resp.Samples) != 2 {
		t.Fatalf("expected 2 samples, got %#v", resp.Samples)
	}
	if got := resp.Samples[1]; got.T != 210 || len(got.S) != 1 || got.S["6"] != 1300 {
		t.Fatalf("unexpected second sample: %#v", got)
	}
	if since := h.History("42", 150); len(since.Samples) != 1 {
		t.Fatalf("since filter: %#v", since.Samples)
	}

	h.observe("42", 230, map[string]int64{"6": 1400})
	if resp := h.History("42", 210); len(resp.Samples) != 1 || resp.Samples[0].S["6"] != 1400 {
		t.Fatalf("pending sample: %#v", resp.Samples)
	}

	if _, err := os.Stat(filepath.Join(dir, "42.jsonl")); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := NewXPHistory(dir, "", nil)
	reloaded.tracked["42"] = &xpTrackedPlayer{Since: 100}
	if got := reloaded.History("42", 0); len(got.Samples) != 2 || got.TrackedSince != 100 {
		t.Fatalf("reload: %#v", got)
	}
}
