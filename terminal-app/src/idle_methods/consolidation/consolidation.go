package consolidation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"terminal-app/src/consolidator"
	"terminal-app/src/embedder"
	"terminal-app/src/summariser"
	"terminal-app/src/utils"

	"github.com/philippgille/chromem-go"
)

// LLMResponse matches the structured JSON expected from the Summariser LLM.
type LLMResponse struct {
	MetricHistory []map[string]string `json:"metric_history"`
	FactArray     []string            `json:"factArray"`
}

// EpisodeSummary is a lightweight view returned after consolidation.
type EpisodeSummary struct {
	ID            string   `json:"id"`
	Facts         []string `json:"facts"`
	PeakMindState string   `json:"peak_mindstate"`
}



// Consolidate reads unsaved messages from history, groups them by character length,
// calls the summariser agent to generate metadata, saves them to episode JSON/CSV files,
// and returns the newly created episode summaries for the runtime memory manager.
func Consolidate(hm *consolidator.HistoryManager) ([]EpisodeSummary, error) {
	messages := hm.GetMessages()
	if len(messages) == 0 {
		return nil, fmt.Errorf("no conversation history to consolidate")
	}

	// Filter indices of unstored messages
	var unstoredIndices []int
	for i, msg := range messages {
		if !msg.Stored {
			unstoredIndices = append(unstoredIndices, i)
		}
	}

	if len(unstoredIndices) == 0 {
		return nil, fmt.Errorf("no new messages to consolidate")
	}

	// Determine character limit for consolidation chunking
	maxChars := 3000
	if limitStr := os.Getenv("SYSTEM_CONSOLIDATION_DENSITY"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			maxChars = limit
		}
	}

	// Group unstored indices into chunks
	var chunks [][]int
	var currentChunk []int
	currentLength := 0

	for _, idx := range unstoredIndices {
		msgLen := len(messages[idx].Content)
		if len(currentChunk) > 0 && currentLength+msgLen > maxChars {
			chunks = append(chunks, currentChunk)
			currentChunk = []int{idx}
			currentLength = msgLen
		} else {
			currentChunk = append(currentChunk, idx)
			currentLength += msgLen
		}
	}
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	// Ensure target directory exists
	episodesDir := utils.ResolvePath(filepath.Join("Context", "episodes"))
	if err := os.MkdirAll(episodesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create episodes directory: %w", err)
	}

	// Initialize the summariser agent
	agent := summariser.NewSummariserAgent()

	var newEpisodes []EpisodeSummary

	// ONLY process the first chunk to spread out API calls and avoid rate limits.
	if len(chunks) > 1 {
		chunks = chunks[:1]
	}

	// Process the chunk
	for chunkIdx, indices := range chunks {
		var chunkMsgIDs []string
		var convBuilder strings.Builder

		// Determine peak mindstate in the chunk based on (Negative + Positive Emotion) activation
		peakMindState := "0.90:0.30:0.50:0.70"
		maxActivation := -1.0

		for _, idx := range indices {
			msg := messages[idx]
			chunkMsgIDs = append(chunkMsgIDs, msg.ID)
			convBuilder.WriteString(fmt.Sprintf("%s: %s\n", msg.Author, msg.Content))

			activation := calculateActivationScore(msg.MindState)
			if activation > maxActivation {
				maxActivation = activation
				if msg.MindState != "" {
					peakMindState = msg.MindState
				}
			}
		}

		// Call Summariser agent to get summary JSON
		rawJSON, err := agent.Summarise(context.Background(), convBuilder.String())
		if err != nil {
			return nil, fmt.Errorf("failed to generate summary for chunk %d: %w", chunkIdx, err)
		}

		var llmResp LLMResponse
		if err := json.Unmarshal([]byte(rawJSON), &llmResp); err != nil {
			// Resilient fallback if LLM output is not valid JSON
			llmResp = LLMResponse{
				MetricHistory: []map[string]string{},
				FactArray:     []string{"Failed to parse model facts. raw response: " + rawJSON},
			}
		}

		episodeID := fmt.Sprintf("%s_ep_%d", hm.SessionID, time.Now().UnixNano())
		deltasStr, _ := json.Marshal(llmResp.MetricHistory)
		timestampStr := time.Now().Format(time.RFC3339)

		// Save to chromem-go
		db, err := chromem.NewPersistentDB(utils.ResolvePath(filepath.Join("Context", "chromem_db")), false)
		if err != nil {
			return nil, fmt.Errorf("failed to init chromem db: %w", err)
		}
		
		emb := embedder.NewLocalEmbedder()
		collection, err := db.GetOrCreateCollection("facts", nil, emb.AsChromemEmbeddingFunc())
		if err != nil {
			return nil, fmt.Errorf("failed to get chromem collection: %w", err)
		}

		var docs []chromem.Document
		for i, factStr := range llmResp.FactArray {
			factID := fmt.Sprintf("%s_fact_%d", episodeID, i)
			docs = append(docs, chromem.Document{
				ID:      factID,
				Content: factStr,
				Metadata: map[string]string{
					"episode_id":    episodeID,
					"timestamp":     timestampStr,
					"metric_deltas": string(deltasStr),
				},
			})
		}
		
		if len(docs) > 0 {
			// Use 1 concurrency as we are in a background process anyway and want to avoid overwhelming local embedding API
			if err := collection.AddDocuments(context.Background(), docs, 1); err != nil {
				fmt.Printf("[DEBUG] Failed to add documents to chromem: %v\n", err)
			}
		}

		// Mark the messages in HistoryManager as stored on disk
		startIdx := indices[0]
		endIdx := indices[len(indices)-1] + 1
		if err := hm.MarkStored(startIdx, endIdx); err != nil {
			return nil, fmt.Errorf("failed to mark messages as stored for episode %s: %w", episodeID, err)
		}

		// Collect the summary for the runtime memory manager
		newEpisodes = append(newEpisodes, EpisodeSummary{
			ID:            episodeID,
			Facts:         llmResp.FactArray,
			PeakMindState: peakMindState,
		})
	}

	return newEpisodes, nil
}

// calculateActivationScore calculates the sum of positive + negative emotion from a mindstate string.
func calculateActivationScore(mindState string) float64 {
	if mindState == "" {
		return 0.0
	}
	var ma, ne, pe, ua float64
	n, err := fmt.Sscanf(mindState, "%f:%f:%f:%f", &ma, &ne, &pe, &ua)
	if err != nil || n < 4 {
		return 0.0
	}
	return ne + pe
}

