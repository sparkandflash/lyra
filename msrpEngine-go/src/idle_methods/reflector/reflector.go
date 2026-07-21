package reflector

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"msrpengine/src/embedder"
	"msrpengine/src/idle_methods/episode_memory"
	"msrpengine/src/responder"
	"msrpengine/src/utils"

	"github.com/philippgille/chromem-go"
)

// Reflect queries the chromem-go facts database using the current context and returns relevant facts as EpisodeSummary objects.
func Reflect(currentMindState string, activeEpisodes []responder.EpisodeSummary) ([]episode_memory.EpisodeSummary, error) {
	_, err := parseAttentionScore(currentMindState)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current mindstate: %w", err)
	}

	var queryBuilder strings.Builder
	for _, ep := range activeEpisodes {
		queryBuilder.WriteString(strings.Join(ep.Facts, " ") + " ")
	}
	queryText := strings.TrimSpace(queryBuilder.String())

	if queryText == "" {
		return nil, nil // No context to query with
	}

	dbPath := utils.ResolvePath(filepath.Join("Context", "chromem_db"))
	db, err := chromem.NewPersistentDB(dbPath, false)
	if err != nil {
		return nil, nil // db probably doesn't exist yet
	}

	emb := embedder.NewLocalEmbedder()
	collection, err := db.GetOrCreateCollection("facts", nil, emb.AsChromemEmbeddingFunc())
	if err != nil {
		return nil, nil
	}

	res, err := collection.Query(context.Background(), queryText, 3, nil, nil)
	if err != nil {
		return nil, err
	}

	var facts []episode_memory.EpisodeSummary
	seenIDs := make(map[string]bool)

	for _, doc := range res {
		if seenIDs[doc.ID] {
			continue
		}
		seenIDs[doc.ID] = true

		facts = append(facts, episode_memory.EpisodeSummary{
			ID:            doc.ID,
			Facts:         []string{doc.Content},
			PeakMindState: currentMindState, // Or extract from metadata
		})

		// Traverse links (1 degree)
		linkedTo := doc.Metadata["linked_to"]
		if linkedTo != "" {
			var linkedFacts []string
			// chromem-go doesn't have an empty query fetch, so iteratively GetByID until failure
			for i := 0; i < 20; i++ {
				factID := fmt.Sprintf("%s_fact_%d", linkedTo, i)
				ldoc, err := collection.GetByID(context.Background(), factID)
				if err != nil {
					break
				}
				linkedFacts = append(linkedFacts, ldoc.Content)
				seenIDs[factID] = true
			}
			
			if len(linkedFacts) > 0 {
				facts = append(facts, episode_memory.EpisodeSummary{
					ID:            linkedTo,
					Facts:         linkedFacts,
					PeakMindState: currentMindState, 
				})
			}
		}
	}

	return facts, nil
}

// parseAttentionScore extracts MA (Model Attention) and UA (User Attention) from a mindstate string
// formatted as "MA:NE:PE:UA" and returns their sum.
func parseAttentionScore(mindState string) (float64, error) {
	parts := strings.Split(mindState, ":")
	if len(parts) != 5 {
		return 0, fmt.Errorf("invalid mindstate format")
	}

	ma, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}
	ua, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, err
	}
	return ma + ua, nil
}
