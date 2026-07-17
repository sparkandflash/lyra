package reflector

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"lyra/responder"
)

// Reflect finds episodes in index.csv with a higher attention score than currentMindState,
// and which share at least one keyword with activeEpisodes.
// currentMindState is formatted as "MA:NE:PE:UA" (e.g. "0.90:0.30:0.50:0.70").
func Reflect(currentMindState string, activeEpisodes []responder.EpisodeSummary) ([]string, error) {
	currentAttn, err := parseAttentionScore(currentMindState)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current mindstate: %w", err)
	}

	// Extract active keywords for filtering
	activeKeywords := make(map[string]bool)
	for _, ep := range activeEpisodes {
		for _, kw := range ep.Keywords {
			activeKeywords[strings.ToLower(strings.TrimSpace(kw))] = true
		}
	}

	// Read index.csv
	indexPath := filepath.Join("Context", "episodes", "index.csv")
	file, err := os.Open(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No episodes yet
		}
		return nil, fmt.Errorf("failed to open index.csv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read index.csv: %w", err)
	}

	if len(records) <= 1 {
		return nil, nil // Only header or empty
	}

	var matchedEpisodeIDs []string
	var highestAttnEpisode string
	var highestAttnScore float64

	// Skip header (row 0)
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) < 3 {
			continue
		}
		rowMindState := row[0]
		rowKeywordsStr := row[1]
		rowEpisodeID := row[2]

		rowAttn, err := parseAttentionScore(rowMindState)
		if err != nil {
			continue
		}

		if rowAttn > highestAttnScore {
			highestAttnScore = rowAttn
			highestAttnEpisode = rowEpisodeID
		}

		// Filter 1: Episode attention must be greater than current attention
		if rowAttn <= currentAttn {
			continue
		}

		// Filter 2: Must share at least one keyword if activeKeywords is not empty
		if len(activeKeywords) > 0 {
			rowKeywords := strings.Split(rowKeywordsStr, ",")
			shared := false
			for _, kw := range rowKeywords {
				cleanKw := strings.ToLower(strings.TrimSpace(kw))
				if activeKeywords[cleanKw] {
					shared = true
					break
				}
			}
			if !shared {
				continue
			}
		}

		matchedEpisodeIDs = append(matchedEpisodeIDs, rowEpisodeID)
	}

	// Fallback: If no matches and active memory is empty, just return the highest attention episode to seed it.
	if len(matchedEpisodeIDs) == 0 && len(activeKeywords) == 0 && highestAttnEpisode != "" && highestAttnScore > currentAttn {
		matchedEpisodeIDs = append(matchedEpisodeIDs, highestAttnEpisode)
	}

	return matchedEpisodeIDs, nil
}

// parseAttentionScore extracts MA (Model Attention) and UA (User Attention) from a mindstate string
// formatted as "MA:NE:PE:UA" and returns their sum.
func parseAttentionScore(mindState string) (float64, error) {
	parts := strings.Split(mindState, ":")
	if len(parts) != 4 {
		return 0, fmt.Errorf("invalid mindstate format")
	}

	ma, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}
	ua, err := strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return 0, err
	}
	return ma + ua, nil
}
