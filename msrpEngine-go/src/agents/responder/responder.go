package responder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"msrpengine/src/agents"
	"msrpengine/src/contextManager"
	"msrpengine/src/prompts"
	"msrpengine/src/utils"
)

// EpisodeSummary is a lightweight episode view passed to the responder LLM as episodic context.
type EpisodeSummary struct {
	ID            string   `json:"id"`
	Facts         []string `json:"facts"`
	PeakMindState string   `json:"peak_mindstate"`
}

// Responder defines the interface for generating responses from LLMs.
type Responder struct {
	agent *agents.Agent
}

// NewResponderFromEnv is kept for compatibility, but now uses utils.Config and the unified agent.
func NewResponderFromEnv() (*Responder, error) {
	agentType := utils.Config.ResponderType
	if agentType == "" {
		agentType = "mock"
	}

	sysPrompt := prompts.GetResponderPrompt(utils.Config.SystemMaxOutputChars)

	agent := agents.NewAgent(
		agentType,
		utils.Config.ResponderAPIKey,
		utils.Config.ResponderBaseURL,
		utils.Config.ResponderModel,
		sysPrompt,
	)

	return &Responder{
		agent: agent,
	}, nil
}

func (r *Responder) Respond(ctx context.Context, prompt string, mindState string, history []contextManager.InterfaceEvent, episodes []EpisodeSummary) (string, string, error) {
	return r.respondInternal(ctx, prompt, mindState, history, episodes, r.agent.SystemPrompt)
}

func (r *Responder) RespondProactive(ctx context.Context, mindState string, history []contextManager.InterfaceEvent, episodes []EpisodeSummary) (string, string, error) {
	systemPrompt := prompts.GetProactivePrompt(utils.Config.SystemMaxOutputChars)
	return r.respondInternal(ctx, "[System: The user has been silent. Initiate conversation.]", mindState, history, episodes, systemPrompt)
}

func (r *Responder) respondInternal(ctx context.Context, prompt string, mindState string, history []contextManager.InterfaceEvent, episodes []EpisodeSummary, systemPrompt string) (string, string, error) {

	// Clean history to remove internal message IDs to prevent LLM confusion
	type cleanHistory struct {
		Author  string `json:"author"`
		Content string `json:"content"`
	}
	cleanedHistory := make([]cleanHistory, len(history))
	for i, h := range history {
		cleanedHistory[i] = cleanHistory{
			Author:  h.Author,
			Content: h.Content,
		}
	}

	userPayload := map[string]interface{}{
		"message":   prompt,
		"mindstate": mindState,
		"history":   cleanedHistory,
		"episodes":  episodes,
	}
	payloadBytes, err := json.Marshal(userPayload)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal user payload: %w", err)
	}

	rawResponse, err := r.agent.Generate(ctx, string(payloadBytes), systemPrompt)
	if err != nil {
		return "", "", err
	}

	reply, episodeID, err := parseResponderOutput(rawResponse)
	return reply, episodeID, err
}

// parseResponderOutput parses the structured JSON the responder LLM is expected to return.
func parseResponderOutput(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	// Strip markdown code fences if present
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}

	var out struct {
		Reply           string `json:"reply"`
		UsefulEpisodeID string `json:"useful_episode_id"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		// Graceful fallback: treat the whole response as plain reply text
		return raw, "", nil
	}
	return out.Reply, out.UsefulEpisodeID, nil
}
