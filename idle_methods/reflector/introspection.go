package reflector

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lyra/idle_methods/consolidation"
	"lyra/prompts"
	"lyra/summariser"
)

// Introspect reads a target episode, sends it to the summariser agent (using the introspection prompt)
// to generate alternative responses, and saves the reflection to disk.
func Introspect(episodeID string) error {
	// 1. Read original episode
	episodesDir := filepath.Join("Context", "episodes")
	episodePath := filepath.Join(episodesDir, fmt.Sprintf("%s.json", episodeID))
	
	data, err := os.ReadFile(episodePath)
	if err != nil {
		return fmt.Errorf("failed to read episode %s: %w", episodeID, err)
	}

	var episode consolidation.Episode
	if err := json.Unmarshal(data, &episode); err != nil {
		return fmt.Errorf("failed to parse episode %s: %w", episodeID, err)
	}

	if len(episode.Messages) == 0 {
		return fmt.Errorf("episode %s has no messages to introspect", episodeID)
	}

	// 2. Format messages for the summariser
	var convBuilder strings.Builder
	for _, msg := range episode.Messages {
		convBuilder.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	// 3. Call SummariserAgent with the Introspection Prompt
	agent := summariser.NewSummariserAgent()
	
	// We need a way to override the system instruction. The SummariserAgent currently loads from config,
	// but we can modify its config temporarily or we can just call its unexported callLLM if we modify it.
	// Since SummariserAgent.Summarise defaults to prompts.GetConsolidationPrompt(), we should probably
	// add a SummariseWithPrompt method or just temporarily change the agent's config.
	// We will temporarily mutate the agent config (if it's exported, but it's not).
	// To avoid modifying summariser package heavily, we can add a new method to summariser if needed,
	// or we can just re-implement a small call here.
	
	// Wait, let's look at summariser package. `SummariserAgent` has `Summarise(ctx, text)`. 
	// We should just use a direct LLM call or modify `summariser.go` to support custom prompts.
	// For now, I'll assume we can call an updated `SummariseWithPrompt` in `summariser`.
	rawJSON, err := agent.SummariseWithPrompt(context.Background(), convBuilder.String(), prompts.GetIntrospectionPrompt())
	if err != nil {
		return fmt.Errorf("introspection LLM call failed: %w", err)
	}

	var llmResp consolidation.LLMResponse
	if err := json.Unmarshal([]byte(rawJSON), &llmResp); err != nil {
		return fmt.Errorf("failed to parse introspection JSON: %w (raw: %s)", err, rawJSON)
	}

	// 4. Save Reflection JSON
	reflectionsDir := filepath.Join(episodesDir, "reflections")
	if err := os.MkdirAll(reflectionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create reflections directory: %w", err)
	}

	reflectionID := fmt.Sprintf("%s_reflection", episodeID)
	reflectionPath := filepath.Join(reflectionsDir, fmt.Sprintf("%s.json", reflectionID))
	
	reflectionData := map[string]interface{}{
		"original_episode_id": episodeID,
		"reflection_summary":  llmResp.Summary,
		"keywords":            llmResp.Keywords,
		"alternative_strategy": llmResp.Conclusion,
	}
	
	rBytes, err := json.MarshalIndent(reflectionData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize reflection: %w", err)
	}
	
	if err := os.WriteFile(reflectionPath, rBytes, 0644); err != nil {
		return fmt.Errorf("failed to write reflection file: %w", err)
	}

	// 5. Append to reflections.csv
	csvPath := filepath.Join(episodesDir, "reflections.csv")
	csvFile, err := os.OpenFile(csvPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open reflections.csv: %w", err)
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	
	// Check if file is empty to write header
	stat, _ := csvFile.Stat()
	if stat.Size() == 0 {
		writer.Write([]string{"original_episodeid", "keywords", "reflectionid"})
	}
	
	writer.Write([]string{
		episodeID,
		strings.Join(llmResp.Keywords, ", "),
		reflectionID,
	})
	writer.Flush()

	return writer.Error()
}
