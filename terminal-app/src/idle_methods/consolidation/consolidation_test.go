package consolidation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"terminal-app/src/consolidator"
)

func TestCalculateActivationScore(t *testing.T) {
	score := calculateActivationScore("0.90:0.80:0.20:0.50")
	if score != 1.00 {
		t.Errorf("expected score 1.00, got %.2f", score)
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
	hm, err := consolidator.NewHistoryManager("")
	if err != nil {
		t.Fatalf("failed to create HistoryManager: %v", err)
	}

	// Setup cleanups
	historyDir := filepath.Join("Context", "conversationHistory")
	defer os.RemoveAll("Context") // recursively delete Context folder in test

	historyFile := filepath.Join(historyDir, hm.SessionID+".json")

	// Save test turns
	err = hm.Save("user", "ping about skrillex dreams", "0.90:0.80:0.20:0.70")
	if err != nil {
		t.Fatalf("failed to save user turn: %v", err)
	}
	err = hm.Save("assistant", "pong with regal tone", "0.90:0.30:0.50:0.70")
	if err != nil {
		t.Fatalf("failed to save assistant turn: %v", err)
	}

	// Trigger consolidation (which will call the mock summariser under the hood since s.config.Type is empty or mock)
	os.Setenv("SYSTEM_MAX_WORKING_MEMORY_CHARS", "100")
	defer os.Unsetenv("SYSTEM_MAX_WORKING_MEMORY_CHARS")

	newEpisodes, err := Consolidate(hm)
	if err != nil {
		t.Fatalf("consolidation failed: %v", err)
	}
	if len(newEpisodes) == 0 {
		t.Error("expected at least one episode summary returned from Consolidate")
	}

	// 1. Verify JSON Episode File was created
	episodePath := filepath.Join("Context", "episodes", newEpisodes[0].ID+".json")
	if _, err := os.Stat(episodePath); os.IsNotExist(err) {
		t.Fatalf("expected episode file to be created at: %s", episodePath)
	}

	// Read and verify episode fields
	epData, err := os.ReadFile(episodePath)
	if err != nil {
		t.Fatalf("failed to read episode file: %v", err)
	}

	var ep Episode
	if err := json.Unmarshal(epData, &ep); err != nil {
		t.Fatalf("failed to unmarshal episode data: %v", err)
	}

	if ep.PeakMindState != "0.90:0.80:0.20:0.70" {
		t.Errorf("expected peak mindstate '0.90:0.80:0.20:0.70', got %q", ep.PeakMindState)
	}
	// 2. Verify JSONL Index File was created
	jsonlPath := filepath.Join("Context", "episodes", "index.jsonl")
	if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
		t.Fatalf("expected index.jsonl to be created")
	}

	// 3. Verify history JSON file messages are updated with stored:true
	histData, err := os.ReadFile(historyFile)
	if err != nil {
		t.Fatalf("failed to read history file: %v", err)
	}
	var histMsgs []consolidator.Message
	if err := json.Unmarshal(histData, &histMsgs); err != nil {
		t.Fatalf("failed to parse history log: %v", err)
	}

	for _, msg := range histMsgs {
		if !msg.Stored {
			t.Errorf("expected all messages to have stored: true, but got: %+v", msg)
		}
	}
}
