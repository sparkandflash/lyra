package consolidation

import (
	"context"
	"os"
	"testing"

	"msrpengine/src/contextManager"
)

func TestCalculateActivationScore(t *testing.T) {
	score := calculateActivationScore("0.90:0.80:0.20:0.50:0.50")
	if score != 1.20 {
		t.Errorf("expected score 1.20, got %.2f", score)
	}

	score = calculateActivationScore("")
	if score != 0.0 {
		t.Errorf("expected score 0.0 for empty mindstate, got %.2f", score)
	}

	score = calculateActivationScore("invalid")
	if score != 0.0 {
		t.Errorf("expected score 0.0 for invalid mindstate, got %.2f", score)
	}
}

func TestConsolidationFlow(t *testing.T) {
	// Setup HistoryManager
	hm, err := contextManager.NewEventLogContext("")
	if err != nil {
		t.Fatalf("failed to create HistoryManager: %v", err)
	}

	// Setup cleanups
	defer os.RemoveAll("Context") // recursively delete Context folder in test


	// Setup IndexManager
	idxMgr, err := contextManager.NewChromemIndexManager()
	if err != nil {
		t.Fatalf("failed to create IndexManager: %v", err)
	}

	metrics1 := contextManager.Metrics{MindScores: "0.90:0.80:0.20:0.70:0.50"}
	// Save test turns
	err = hm.Save("user", "ping about skrillex dreams", metrics1)
	if err != nil {
		t.Fatalf("failed to save user turn: %v", err)
	}
	metrics2 := contextManager.Metrics{MindScores: "0.90:0.30:0.50:0.70:0.50"}
	err = hm.Save("assistant", "pong with regal tone", metrics2)
	if err != nil {
		t.Fatalf("failed to save assistant turn: %v", err)
	}

	// Trigger consolidation (which will call the mock summariser under the hood since s.config.Type is empty or mock)
	os.Setenv("SYSTEM_MAX_WORKING_MEMORY_CHARS", "100")
	defer os.Unsetenv("SYSTEM_MAX_WORKING_MEMORY_CHARS")

	newEpisodes, err := Consolidate(context.Background(), hm, idxMgr, nil)
	if err != nil {
		t.Fatalf("consolidation failed: %v", err)
	}
	if len(newEpisodes) == 0 {
		t.Error("expected at least one episode summary returned from Consolidate")
	}

	// 1. Verify EpisodeSummary was returned
	if len(newEpisodes[0].Facts) == 0 {
		t.Errorf("expected facts to be extracted")
	}
	
	if newEpisodes[0].PeakMindState != "0.90:0.30:0.50:0.70:0.50" {
		t.Errorf("expected peak mindstate '0.90:0.30:0.50:0.70:0.50', got %q", newEpisodes[0].PeakMindState)
	}

	// 3. Verify messages are marked as consolidated in the index
	histMsgs := hm.GetMessages()
	for _, msg := range histMsgs {
		if !idxMgr.IsMessageConsolidated(msg.ID) {
			t.Errorf("expected all messages to be marked consolidated, but got: %+v", msg)
		}
	}
}
