package consolidation

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lyra/consolidator"
	"lyra/summariser"
)

// Episode represents the JSON structure of a consolidated episodic memory.
type Episode struct {
	ID            string                 `json:"id"`
	Summary       string                 `json:"summary"`
	Keywords      []string               `json:"keywords"`
	PeakMindState string                 `json:"peak_mindstate"`
	Conclusion    string                 `json:"conclusion"`
	Messages      []consolidator.Message `json:"messages"`
}

// LLMResponse matches the structured JSON expected from the Summariser LLM.
type LLMResponse struct {
	Summary    string   `json:"summary"`
	Keywords   []string `json:"keywords"`
	Conclusion string   `json:"conclusion"`
}

// EpisodeSummary is a lightweight, metadata-only view of an episode returned after consolidation.
// It contains no raw message data — only enough for the runtime episode memory manager.
type EpisodeSummary struct {
	ID            string   `json:"id"`
	Summary       string   `json:"summary"`
	Keywords      []string `json:"keywords"`
	PeakMindState string   `json:"peak_mindstate"`
	Conclusion    string   `json:"conclusion"`
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
	if limitStr := os.Getenv("LYRA_CONSOLIDATION_DENSITY"); limitStr != "" {
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
	episodesDir := filepath.Join("Context", "episodes")
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
		var chunkMsgs []consolidator.Message
		var convBuilder strings.Builder

		// Determine peak mindstate in the chunk based on (Negative + Positive Emotion) activation
		peakMindState := "0.90:0.30:0.50:0.70"
		maxActivation := -1.0

		for _, idx := range indices {
			msg := messages[idx]
			chunkMsgs = append(chunkMsgs, msg)
			convBuilder.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))

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
				Summary:    "Failed to parse model summary. raw response: " + rawJSON,
				Keywords:   []string{"parsed-error"},
				Conclusion: "Parsed error conclusion.",
			}
		}

		// Save Episode JSON. Use UnixNano to ensure uniqueness across multiple consolidation runs.
		episodeID := fmt.Sprintf("%s_ep_%d", hm.SessionID, time.Now().UnixNano())
		episode := Episode{
			ID:            episodeID,
			Summary:       llmResp.Summary,
			Keywords:      llmResp.Keywords,
			PeakMindState: peakMindState,
			Conclusion:    llmResp.Conclusion,
			Messages:      chunkMsgs,
		}

		episodePath := filepath.Join(episodesDir, fmt.Sprintf("%s.json", episodeID))
		episodeData, err := json.MarshalIndent(episode, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to serialize episode %s: %w", episodeID, err)
		}

		if err := os.WriteFile(episodePath, episodeData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write episode file %s: %w", episodePath, err)
		}

		// Append to central CSV Index
		if err := appendToIndexCSV(episodesDir, peakMindState, llmResp.Keywords, episodeID); err != nil {
			return nil, fmt.Errorf("failed to write to index CSV: %w", err)
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
			Summary:       llmResp.Summary,
			Keywords:      llmResp.Keywords,
			PeakMindState: peakMindState,
			Conclusion:    llmResp.Conclusion,
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

// appendToIndexCSV writes the episode entry into the central episodes CSV index.
func appendToIndexCSV(dir, peakMindState string, keywords []string, episodeID string) error {
	csvPath := filepath.Join(dir, "index.csv")
	fileExisted := true
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		fileExisted = false
	}

	file, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if !fileExisted {
		// Write headers
		if err := writer.Write([]string{"mindstatescore", "keywords", "episodeid"}); err != nil {
			return err
		}
	}

	kwJoined := strings.Join(keywords, ", ")
	row := []string{peakMindState, kwJoined, episodeID}
	return writer.Write(row)
}
