package episode_memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPushAndEviction(t *testing.T) {
	// ep1=108 bytes, ep2=109 bytes serialized (total 217). ep3=171 bytes.
	// Budget of 250: ep1+ep2 fit (217 < 250).
	// After pushing ep3: 217+171=388 > 250, ep1 is evicted first (109+171=280 still > 250),
	// then ep2 is evicted (171 <= 250). Only ep3 remains.
	mgr := NewEpisodeMemoryManager(170)

	ep1 := EpisodeSummary{ID: "ep1", Facts: []string{"first episode"}, PeakMindState: "0.9:0.3:0.5:0.7:0.5"}
	ep2 := EpisodeSummary{ID: "ep2", Facts: []string{"second episode"}, PeakMindState: "0.9:0.3:0.5:0.7:0.5"}
	ep3 := EpisodeSummary{ID: "ep3", Facts: []string{"third episode with more content to push over limit"}, PeakMindState: "0.9:0.3:0.5:0.7:0.5"}

	mgr.Push(ep1)
	mgr.Push(ep2)
	if len(mgr.GetActive()) != 2 {
		t.Fatalf("expected 2 active episodes after 2 pushes, got %d", len(mgr.GetActive()))
	}

	// Adding ep3 (171 bytes) to ep1(108)+ep2(109)=388 bytes total, exceeds the 250-char budget.
	// Oldest non-pinned episodes (ep1, then ep2) are evicted until within budget.
	mgr.Push(ep3)
	active := mgr.GetActive()

	// ep1 and ep2 should have been evicted; ep3 remains
	for _, ep := range active {
		if ep.ID == "ep1" || ep.ID == "ep2" {
			t.Errorf("expected ep1 and ep2 to be evicted, but %q is still in active pool", ep.ID)
		}
	}
	if len(active) != 1 || active[0].ID != "ep3" {
		t.Errorf("expected only ep3 to remain, got %d episodes: %+v", len(active), active)
	}
}

func TestPinPreventsEviction(t *testing.T) {
	// ep1(108)+ep2(109)=217 < 300, both fit. With ep1 pinned and ep3(171) added:
	// Total 388 > 300 → evict oldest non-pinned = ep2. ep1+ep3=279 < 300, done.
	mgr := NewEpisodeMemoryManager(180)

	ep1 := EpisodeSummary{ID: "ep1", Facts: []string{"first episode"}, PeakMindState: "0.9:0.3:0.5:0.7:0.5"}
	ep2 := EpisodeSummary{ID: "ep2", Facts: []string{"second episode"}, PeakMindState: "0.9:0.3:0.5:0.7:0.5"}
	ep3 := EpisodeSummary{ID: "ep3", Facts: []string{"third episode with more content to push over limit"}, PeakMindState: "0.9:0.3:0.5:0.7:0.5"}

	mgr.Push(ep1)
	mgr.MarkUseful("ep1") // pin ep1

	mgr.Push(ep2)
	mgr.Push(ep3) // should evict ep2, not ep1

	active := mgr.GetActive()
	ep1Found := false
	ep2Found := false
	for _, ep := range active {
		if ep.ID == "ep1" {
			ep1Found = true
		}
		if ep.ID == "ep2" {
			ep2Found = true
		}
	}

	if !ep1Found {
		t.Errorf("ep1 is pinned and should NOT have been evicted")
	}
	if ep2Found {
		t.Errorf("ep2 is unpinned and should have been evicted to make room")
	}
}

func TestLoadFromDisk(t *testing.T) {
	// Write a temporary episode JSON file
	dir := t.TempDir()
	epFile := filepath.Join(dir, "test_ep_1.json")
	content := `{
		"id": "test_ep_1",
		"facts": ["test summary"],
		"peak_mindstate": "0.9:0.3:0.5:0.7:0.5"
	}`
	if err := os.WriteFile(epFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test episode file: %v", err)
	}

	mgr := NewEpisodeMemoryManager(5000)
	if err := mgr.LoadFromDisk(epFile); err != nil {
		t.Fatalf("LoadFromDisk failed: %v", err)
	}

	active := mgr.GetActive()
	if len(active) != 1 {
		t.Fatalf("expected 1 episode in pool, got %d", len(active))
	}
	if active[0].ID != "test_ep_1" {
		t.Errorf("expected episode ID 'test_ep_1', got %q", active[0].ID)
	}
	if len(active[0].Facts) != 1 || active[0].Facts[0] != "test summary" {
		t.Errorf("expected summary 'test summary', got %v", active[0].Facts)
	}
}

func TestMarkUseful(t *testing.T) {
	mgr := NewEpisodeMemoryManager(5000)
	mgr.MarkUseful("ep42")
	if mgr.GetPinnedID() != "ep42" {
		t.Errorf("expected pinned ID 'ep42', got %q", mgr.GetPinnedID())
	}
	mgr.MarkUseful("")
	if mgr.GetPinnedID() != "" {
		t.Errorf("expected pinned ID to be cleared, got %q", mgr.GetPinnedID())
	}
}
