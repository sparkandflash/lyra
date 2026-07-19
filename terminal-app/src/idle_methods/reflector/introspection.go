package reflector

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"terminal-app/src/consolidator"
	"terminal-app/src/embedder"
	"terminal-app/src/idle_methods/episode_memory"
	"terminal-app/src/prompts"
	"terminal-app/src/summariser"
	"terminal-app/src/utils"

	"github.com/philippgille/chromem-go"
)

// Introspect reads the conversation history, searches for high negative emotion interactions,
// checks if they are semantically related to the current context, and if so,
// generates a behavioral strategy fact and saves it to chromem-go.
func Introspect(hm *consolidator.HistoryManager, episodeMgr *episode_memory.EpisodeMemoryManager) error {
	messages := hm.GetMessages()
	if len(messages) == 0 {
		return fmt.Errorf("no conversation history to introspect")
	}

	var targetIdx int = -1
	// 1. Context Filtering: Scan history for high Negative Emotion (NE)
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.MindState != "" {
			parts := strings.Split(msg.MindState, ":")
			if len(parts) >= 5 {
				ne, err := strconv.ParseFloat(parts[4], 64) // Cortisol
				if err == nil && ne > 0.6 {
					targetIdx = i
					break
				}
			}
		}
	}

	if targetIdx == -1 {
		return fmt.Errorf("no highly negative interactions found to introspect on")
	}

	targetMsg := messages[targetIdx]

	// 2. Semantic Filter: Check if targetMsg is semantically related to current active facts
	activeEps := episodeMgr.GetActive()
	if len(activeEps) > 0 {
		emb := embedder.NewLocalEmbedder()
		msgEmb, err := emb.Embed(context.Background(), targetMsg.Content)
		if err == nil {
			var maxSim float64
			for _, ep := range activeEps {
				for _, fact := range ep.Facts {
					factEmb, err := emb.Embed(context.Background(), fact)
					if err == nil {
						sim := embedder.CosineSimilarity(msgEmb, factEmb)
						if sim > maxSim {
							maxSim = sim
						}
					}
				}
			}
			// If the highest similarity is too low, skip introspection
			if maxSim < 0.6 {
				return fmt.Errorf("highest semantic similarity to current context (%.2f) is below threshold, skipping introspection", maxSim)
			}
		}
	}

	// 3. Extract the conversation chunk (e.g., 2 messages before, 2 messages after)
	startIdx := targetIdx - 2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := targetIdx + 2
	if endIdx >= len(messages) {
		endIdx = len(messages) - 1
	}

	var sb strings.Builder
	for i := startIdx; i <= endIdx; i++ {
		sb.WriteString(fmt.Sprintf("%s: %s\n", messages[i].Author, messages[i].Content))
	}
	transcript := sb.String()

	// 4. Call SummariserAgent
	summariserAgent := summariser.NewSummariserAgent()

	sysPrompt := prompts.GetIntrospectionPrompt()
	respStr, err := summariserAgent.SummariseWithPrompt(context.Background(), transcript, sysPrompt)
	if err != nil {
		return fmt.Errorf("failed to call summariser for introspection: %w", err)
	}

	// 5. Parse behavioral_fact
	var result struct {
		BehavioralFact string `json:"behavioral_fact"`
	}
	if err := json.Unmarshal([]byte(respStr), &result); err != nil {
		return fmt.Errorf("failed to parse introspection json: %w\nResponse was: %s", err, respStr)
	}
	if result.BehavioralFact == "" {
		return fmt.Errorf("no behavioral fact generated")
	}

	factStr := "[BEHAVIORAL STRATEGY] " + result.BehavioralFact

	// 6. Insert into chromem-go
	db, err := chromem.NewPersistentDB(utils.ResolvePath(filepath.Join("Context", "chromem_db")), false)
	if err != nil {
		return fmt.Errorf("failed to init chromem db: %w", err)
	}
	
	emb := embedder.NewLocalEmbedder()
	collection, err := db.GetOrCreateCollection("facts", nil, emb.AsChromemEmbeddingFunc())
	if err != nil {
		return fmt.Errorf("failed to get chromem collection: %w", err)
	}

	timestampStr := strconv.FormatInt(time.Now().Unix(), 10)
	docID := fmt.Sprintf("introspect_fact_%s", timestampStr)
	doc := chromem.Document{
		ID:      docID,
		Content: factStr,
		Metadata: map[string]string{
			"timestamp": timestampStr,
			"source":    "introspection",
		},
	}

	if err := collection.AddDocuments(context.Background(), []chromem.Document{doc}, 1); err != nil {
		return fmt.Errorf("failed to add behavioral fact to chromem: %w", err)
	}

	return nil
}
