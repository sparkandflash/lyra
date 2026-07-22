package reactor

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"msrpengine/src/agents"
	"msrpengine/src/contextManager"
	"msrpengine/src/prompts"
	"msrpengine/src/utils"
)

// ReactorResponse represents the structured JSON output expected from the LLM.
type ReactorResponse struct {
	ModelAttention float64 `json:"model_attention"`
	UserAttention  float64 `json:"user_attention"`
	Serotonin      float64 `json:"serotonin"`
	Oxytocin       float64 `json:"oxytocin"`
	Cortisol       float64 `json:"cortisol"`
}

// ReactorAgent analyzes conversation history to output heart rate adjustments.
type ReactorAgent struct {
	agent *agents.Agent
}

// NewReactorAgent creates and configures a new ReactorAgent.
func NewReactorAgent() *ReactorAgent {
	agentType := utils.Config.ReactorType
	if agentType == "" {
		agentType = "mock"
	}

	sysPrompt := prompts.GetReactorPrompt()

	agent := agents.NewAgent(
		agentType,
		utils.Config.ReactorAPIKey,
		utils.Config.ReactorBaseURL,
		utils.Config.ReactorModel,
		sysPrompt,
	)

	return &ReactorAgent{
		agent: agent,
	}
}

// React decides the mindstate scores based on the conversation history.
func (r *ReactorAgent) React(ctx context.Context, history []contextManager.InterfaceEvent) (ReactorResponse, error) {
	if len(history) == 0 {
		return ReactorResponse{ModelAttention: 0.1, UserAttention: 0.7, Serotonin: 0.1, Oxytocin: 0.1, Cortisol: 0.1}, nil
	}

	// Use mock behavior if using mock responder or if no keys are configured.
	if r.agent.Type == "mock" || r.agent.Type == "" {
		return r.reactMock(history)
	}

	// Format conversation history for LLM review
	historyBytes, err := json.Marshal(history)
	if err != nil {
		return ReactorResponse{ModelAttention: 0.1, UserAttention: 0.7, Serotonin: 0.1, Oxytocin: 0.1, Cortisol: 0.1}, fmt.Errorf("failed to marshal history for reactor: %w", err)
	}

	rawResponse, err := r.agent.Generate(ctx, string(historyBytes), r.agent.SystemPrompt)
	if err != nil {
		return ReactorResponse{ModelAttention: 0.1, UserAttention: 0.7, Serotonin: 0.1, Oxytocin: 0.1, Cortisol: 0.1}, fmt.Errorf("reactor LLM call failed: %w", err)
	}

	cleanedJSON := cleanJSONResponse(rawResponse)
	var resp ReactorResponse
	if err := json.Unmarshal([]byte(cleanedJSON), &resp); err != nil {
		// Fallback: robust regex parsing if the LLM hallucinates malformed JSON
		extractFloat := func(key string) float64 {
			idx := strings.Index(rawResponse, key)
			if idx == -1 {
				return -1.0
			}
			sub := rawResponse[idx+len(key):]
			re := regexp.MustCompile(`[-]?[0-9]+\.[0-9]+`)
			match := re.FindString(sub)
			if match == "" {
				return -1.0
			}
			val, _ := strconv.ParseFloat(match, 64)
			return val
		}

		ma := extractFloat("model_attention")
		ua := extractFloat("user_attention")
		se := extractFloat("serotonin")
		ox := extractFloat("oxytocin")
		co := extractFloat("cortisol")

		if ma >= -1.0 && ua >= -1.0 && se >= -1.0 && ox >= -1.0 && co >= -1.0 {
			resp.ModelAttention = ma
			resp.UserAttention = ua
			resp.Serotonin = se
			resp.Oxytocin = ox
			resp.Cortisol = co
		} else {
			return ReactorResponse{ModelAttention: 0.1, UserAttention: 0.7, Serotonin: 0.1, Oxytocin: 0.1, Cortisol: 0.1}, fmt.Errorf("failed to parse reactor JSON output %q: %w", cleanedJSON, err)
		}
	}

	clampBi := func(val float64) float64 {
		if val < -1.0 {
			return -1.0
		}
		if val > 1.0 {
			return 1.0
		}
		return val
	}

	return ReactorResponse{
		ModelAttention: clampBi(resp.ModelAttention),
		UserAttention:  clampBi(resp.UserAttention),
		Serotonin:      clampBi(resp.Serotonin),
		Oxytocin:       clampBi(resp.Oxytocin),
		Cortisol:       clampBi(resp.Cortisol),
	}, nil
}

// reactMock analyzes keywords and message lengths to simulate dynamic mindstate scores offline.
func (r *ReactorAgent) reactMock(history []contextManager.InterfaceEvent) (ReactorResponse, error) {
	// Initialize default state
	resp := ReactorResponse{
		ModelAttention: 0.1,
		UserAttention:  0.7,
		Serotonin:      0.1,
		Oxytocin:       0.1,
		Cortisol:       0.1,
	}

	if len(history) == 0 {
		return resp, nil
	}

	// 1. Evaluate user attention and emotion based on last user message
	var lastUserMsg string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Author == "user" {
			lastUserMsg = history[i].Content
			break
		}
	}

	if lastUserMsg != "" {
		lastUserMsgLower := strings.ToLower(lastUserMsg)

		// Dry vs chatty heuristic for user attention
		if len(lastUserMsg) > 80 {
			resp.UserAttention = 0.9
		} else if len(lastUserMsg) < 20 {
			resp.UserAttention = 0.3
		} else {
			resp.UserAttention = 0.6
		}

		negativeKeywords := []string{"angry", "hate", "mad", "terrible", "worst", "sarcastic", "resentful", "cursed", "passive aggressive", "roast", "suffer"}
		positiveKeywords := []string{"happy", "excited", "love", "great", "awesome", "amazing", "wow", "cool", "pride", "nice"}
		scaryKeywords := []string{"stop", "delete", "die", "kill", "shut up", "warning"}

		for _, word := range negativeKeywords {
			if strings.Contains(lastUserMsgLower, word) {
				resp.Cortisol = 0.8    // Stress
				resp.Serotonin = -0.5 // Sad
				break
			}
		}

		for _, word := range positiveKeywords {
			if strings.Contains(lastUserMsgLower, word) {
				resp.Serotonin = 0.8 // Happy
				resp.Cortisol = -0.5 // Relaxed
				break
			}
		}

		for _, word := range scaryKeywords {
			if strings.Contains(lastUserMsgLower, word) {
				resp.Oxytocin = -0.8 // Fear (Low trust)
				resp.Cortisol = 0.9  // Extreme stress
				break
			}
		}
	}

	// 2. Evaluate model attention based on last assistant message
	var lastAssistantMsg string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Author != "user" && history[i].Author != "system" {
			lastAssistantMsg = history[i].Content
			break
		}
	}

	if lastAssistantMsg != "" {
		if len(lastAssistantMsg) > 120 {
			resp.ModelAttention = 0.95
		} else if len(lastAssistantMsg) < 30 {
			resp.ModelAttention = 0.40
		} else {
			resp.ModelAttention = 0.75
		}
	}

	return resp, nil
}

// cleanJSONResponse strips markdown fences, non-breaking spaces, and any
// surrounding garbage, then extracts the first balanced {...} JSON object.
func cleanJSONResponse(raw string) string {
	// 1. Normalize unicode whitespace (e.g. \u00a0 non-breaking spaces → regular space)
	raw = strings.Map(func(r rune) rune {
		if r == '\u00a0' || r == '\u200b' || r == '\ufeff' {
			return ' '
		}
		return r
	}, raw)

	// 2. Strip markdown code fences
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimSuffix(raw, "```")
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
	}
	raw = strings.TrimSpace(raw)

	// 3. Extract the first balanced {...} block (handles garbage before/after JSON)
	start := strings.Index(raw, "{")
	if start == -1 {
		return raw
	}
	depth := 0
	for i := start; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(raw[start : i+1])
			}
		}
	}
	// Fell off the end — return what we found from start
	return strings.TrimSpace(raw[start:])
}
