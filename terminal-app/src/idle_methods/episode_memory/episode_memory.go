package episode_memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EpisodeSummary is a lightweight view of an episode — no raw messages, just metadata.
// This is what gets sent to the responder LLM and kept in runtime memory.
type EpisodeSummary struct {
	ID            string   `json:"id"`
	Facts         []string `json:"facts"`
	PeakMindState string   `json:"peak_mindstate"`
}

// episodeOnDisk mirrors the on-disk JSON structure so we can load full episodes.
// We only read this; we never write to it.
type episodeOnDisk struct {
	ID            string   `json:"id"`
	Facts         []string `json:"facts"`
	PeakMindState string   `json:"peak_mindstate"`
	// Messages field intentionally ignored — not needed in runtime memory
}

// EpisodeMemoryManager is a runtime-only in-memory pool of episode summaries.
// Episode JSON files are never modified by this manager.
type EpisodeMemoryManager struct {
	active          []EpisodeSummary
	maxChars        int
	pinnedEpisodeID string
}

// NewEpisodeMemoryManager creates a manager with the given character budget.
func NewEpisodeMemoryManager(maxChars int) *EpisodeMemoryManager {
	return &EpisodeMemoryManager{
		active:   []EpisodeSummary{},
		maxChars: maxChars,
	}
}

// LoadEpisodeMemoryManagerFromEnv reads SYSTEM_EPISODE_MEMORY_CHARS (default 2000)
// and returns a configured EpisodeMemoryManager.
func LoadEpisodeMemoryManagerFromEnv() *EpisodeMemoryManager {
	maxChars := 2000
	if limitStr := os.Getenv("SYSTEM_EPISODE_MEMORY_CHARS"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			maxChars = limit
		}
	}
	return NewEpisodeMemoryManager(maxChars)
}

// Push adds a new episode to the active pool.
// If the pool exceeds maxChars after adding, the oldest non-pinned episode is evicted.
// Episode JSON files are never touched.
func (m *EpisodeMemoryManager) Push(ep EpisodeSummary) {
	m.active = append(m.active, ep)

	// Evict oldest non-pinned episodes until within budget
	for m.totalChars() > m.maxChars && len(m.active) > 0 {
		evicted := false
		for i, e := range m.active {
			if e.ID != m.pinnedEpisodeID {
				m.active = append(m.active[:i], m.active[i+1:]...)
				evicted = true
				break
			}
		}
		// Safety: if all episodes are pinned and we still exceed, stop to avoid infinite loop
		if !evicted {
			break
		}
	}
}

// LoadFromDisk reads an episode JSON file from disk and pushes the summary into the pool.
// The episode file is opened read-only and is never modified.
func (m *EpisodeMemoryManager) LoadFromDisk(episodePath string) error {
	data, err := os.ReadFile(episodePath)
	if err != nil {
		return fmt.Errorf("failed to read episode file %q: %w", episodePath, err)
	}

	var ep episodeOnDisk
	if err := json.Unmarshal(data, &ep); err != nil {
		return fmt.Errorf("failed to parse episode file %q: %w", episodePath, err)
	}

	m.Push(EpisodeSummary{
		ID:            ep.ID,
		Facts:         ep.Facts,
		PeakMindState: ep.PeakMindState,
	})
	return nil
}

// MarkUseful pins an episode by ID so it is not evicted by Push.
// Pass an empty string to clear the pin.
func (m *EpisodeMemoryManager) MarkUseful(episodeID string) {
	m.pinnedEpisodeID = strings.TrimSpace(episodeID)
}

// GetActive returns the current slice of active episode summaries.
func (m *EpisodeMemoryManager) GetActive() []EpisodeSummary {
	return m.active
}

// GetPinnedID returns the currently pinned episode ID (empty if none).
func (m *EpisodeMemoryManager) GetPinnedID() string {
	return m.pinnedEpisodeID
}

// totalChars calculates the total character count of all serialized active episodes.
func (m *EpisodeMemoryManager) totalChars() int {
	total := 0
	for _, ep := range m.active {
		b, err := json.Marshal(ep)
		if err == nil {
			total += len(b)
		}
	}
	return total
}

// LoadLatestFromDir loads episode JSON files from a directory, in filename order (oldest first).
// Only files with the suffix "_ep_<N>.json" are loaded.
// This is a convenience helper for loading prior context on boot.
func (m *EpisodeMemoryManager) LoadLatestFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No episodes directory yet — not an error
		}
		return fmt.Errorf("failed to read episodes directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "index.csv" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		// Ignore load errors for individual files — log silently
		_ = m.LoadFromDisk(path)
	}
	return nil
}
