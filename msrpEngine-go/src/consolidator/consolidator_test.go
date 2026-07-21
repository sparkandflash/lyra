package consolidator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSTMmanagerCapping(t *testing.T) {
	// Setup STM with a small character limit of 10
	stm := NewSTMmanager(10)

	// Add a message of 6 characters: "hello!"
	stm.Update("user", "hello!")
	msgs := stm.Get()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	// Add another message of 6 characters: "world!" (total = 12, which exceeds 10)
	stm.Update("assistant", "world!")
	msgs = stm.Get()
	// Total count is 12, so the first message "hello!" should have been pruned.
	if len(msgs) != 1 {
		t.Fatalf("expected first message to be pruned, got length %d", len(msgs))
	}
	if msgs[0].Content != "world!" {
		t.Errorf("expected remaining message to be 'world!', got %q", msgs[0].Content)
	}

	// Test adding a message that is itself larger than maxChars
	stm.Update("user", "thisIsTooLong") // 13 characters
	msgs = stm.Get()
	// Should prune all messages including the new one because it exceeds 10
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages when single input exceeds limit, got %d", len(msgs))
	}
}

func TestHistoryManager(t *testing.T) {
	// Test NewHistoryManager
	hm, err := NewHistoryManager("")
	if err != nil {
		t.Fatalf("failed to initialize HistoryManager: %v", err)
	}

	// Verify directory and file creation
	dir := filepath.Join("Context", "conversationHistory")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatalf("history directory was not created")
	}

	filePath := filepath.Join(dir, hm.SessionID+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("session file was not created: %s", filePath)
	}
	defer os.Remove(filePath) // Cleanup test file

	// Save some chat turns
	err = hm.Save("user", "ping", "0.90:0.30:0.50:0.70:0.50")
	if err != nil {
		t.Fatalf("failed to save user message: %v", err)
	}
	err = hm.Save("assistant", "pong", "0.90:0.30:0.50:0.70:0.50")
	if err != nil {
		t.Fatalf("failed to save assistant message: %v", err)
	}

	// Read file and parse JSON to verify contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read written history file: %v", err)
	}

	var msgs []Message
	err = json.Unmarshal(data, &msgs)
	if err != nil {
		t.Fatalf("failed to parse logged history JSON: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages logged, got %d", len(msgs))
	}
	if msgs[0].Author != "user" || msgs[0].Content != "ping" || msgs[0].MindState != "0.90:0.30:0.50:0.70:0.50" || msgs[0].Stored {
		t.Errorf("unexpected first logged message: %+v", msgs[0])
	}
	if msgs[1].Author != "assistant" || msgs[1].Content != "pong" || msgs[1].MindState != "0.90:0.30:0.50:0.70:0.50" || msgs[1].Stored {
		t.Errorf("unexpected second logged message: %+v", msgs[1])
	}

	// Test MarkStored
	err = hm.MarkStored(0, 2)
	if err != nil {
		t.Fatalf("failed to mark messages as stored: %v", err)
	}

	// Re-verify from disk
	data, err = os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read written history file again: %v", err)
	}
	err = json.Unmarshal(data, &msgs)
	if err != nil {
		t.Fatalf("failed to parse logged history JSON again: %v", err)
	}

	if !msgs[0].Stored || !msgs[1].Stored {
		t.Errorf("expected messages to be marked as stored, got: %+v, %+v", msgs[0], msgs[1])
	}
}
