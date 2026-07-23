package contextManager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"msrpengine/src/utils"
)

type Metrics struct {
	EnergyLevel     float64 `json:"energy_level"`
	EnergyDrainRate float64 `json:"energy_drain_rate"`
	MindScores      string  `json:"mind_scores"`
}

// Message represents a single chat turn with an ID, role, content, and metrics.
type InterfaceEvent struct {
	ID      string  `json:"id"`
	Author  string  `json:"author"`
	Content string  `json:"content"`
	Metrics Metrics `json:"metrics,omitempty"`
}

// ShortTermContext manages the rolling short term memory of the chat.
type ShortTermContext struct {
	mu       sync.RWMutex
	maxChars int
	messages []InterfaceEvent
}

// NewShortTermContext initializes a new ShortTermContext with a maximum character limit.
func NewShortTermContext(maxChars int) *ShortTermContext {
	return &ShortTermContext{
		maxChars: maxChars,
		messages: []InterfaceEvent{},
	}
}

// Get returns all messages currently stored in short term memory.
func (m *ShortTermContext) Get() []InterfaceEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := make([]InterfaceEvent, len(m.messages))
	copy(cp, m.messages)
	return cp
}

// GetNoFlags returns all messages in STM with only ID, Author, and Content populated.
// MindState and Stored flags are omitted — this is the clean view sent to the responder LLM.
func (m *ShortTermContext) GetNoFlags() []InterfaceEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	clean := make([]InterfaceEvent, len(m.messages))
	for i, msg := range m.messages {
		clean[i] = InterfaceEvent{ID: msg.ID, Author: msg.Author, Content: msg.Content}
	}
	return clean
}

// Update appends a message and discards older ones (FIFO) until the total character length is within the maxChars limit.
func (m *ShortTermContext) Update(role string, content string) {
	msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = append(m.messages, InterfaceEvent{ID: msgID, Author: role, Content: content})

	// FIFO pruning based on the character length of the contents
	for m.totalChars() > m.maxChars && len(m.messages) > 0 {
		m.messages = m.messages[1:]
	}
}

// totalChars calculates the sum of characters of all messages in short term memory.
func (m *ShortTermContext) totalChars() int {
	// Callers must hold lock
	sum := 0
	for _, msg := range m.messages {
		sum += len(msg.Content)
	}
	return sum
}

// EventLogContext manages the persistent log of the full conversation.
type EventLogContext struct {
	mu        sync.RWMutex
	SessionID string
	filePath  string
	messages  []InterfaceEvent
}

// NewEventLogContext initializes a persistent history manager for a given SessionID.
// If the session file already exists, it loads the historical messages into memory.
func NewEventLogContext(sessionID string) (*EventLogContext, error) {
	if sessionID == "" {
		sessionID = time.Now().Format("20060102-150405")
	}

	dir := utils.ResolvePath(filepath.Join("Context", "interfaceEventLog"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create interfaceEventLog directory: %w", err)
	}

	filePath := filepath.Join(dir, fmt.Sprintf("%s.json", sessionID))

	messages := []InterfaceEvent{}
	
	// If the file exists, attempt to load it
	if _, err := os.Stat(filePath); err == nil {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read existing history file: %w", err)
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &messages); err != nil {
				return nil, fmt.Errorf("failed to parse existing history file: %w", err)
			}
		}
	} else {
		// Initialize the file as an empty JSON array
		if err := os.WriteFile(filePath, []byte("[]"), 0644); err != nil {
			return nil, fmt.Errorf("failed to initialize history file: %w", err)
		}
	}

	return &EventLogContext{
		SessionID: sessionID,
		filePath:  filePath,
		messages:  messages,
	}, nil
}

// Save appends a new message to the persistent history and writes the full log to disk.
func (h *EventLogContext) Save(role string, content string, metrics Metrics) error {
	msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, InterfaceEvent{ID: msgID, Author: role, Content: content, Metrics: metrics})

	data, err := json.MarshalIndent(h.messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize history: %w", err)
	}

	if err := os.WriteFile(h.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write history to disk: %w", err)
	}

	return nil
}

// GetMessages returns a copy of all messages currently stored in the history manager.
func (h *EventLogContext) GetMessages() []InterfaceEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()
	cp := make([]InterfaceEvent, len(h.messages))
	copy(cp, h.messages)
	return cp
}

