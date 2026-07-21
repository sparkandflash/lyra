package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"msrpengine/src/consolidator"
	"msrpengine/src/prompts"
	"msrpengine/src/responder"
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
	config responder.Config
}

// NewReactorAgent creates and configures a new ReactorAgent.
func NewReactorAgent() *ReactorAgent {
	return &ReactorAgent{
		config: responder.LoadReactorConfigFromEnv(),
	}
}

// React decides the mindstate scores based on the conversation history.
func (r *ReactorAgent) React(ctx context.Context, history []consolidator.Message) (ReactorResponse, error) {
	if len(history) == 0 {
		return ReactorResponse{ModelAttention: 0.1, UserAttention: 0.7, Serotonin: 0.1, Oxytocin: 0.1, Cortisol: 0.1}, nil
	}

	// Use mock behavior if using mock responder or if no keys are configured.
	if r.config.Type == "mock" || r.config.Type == "" || (r.config.Type == "gemini" && r.config.APIKey == "") || (r.config.Type == "openai" && r.config.APIKey == "" && !strings.Contains(r.config.BaseURL, "localhost") && !strings.Contains(r.config.BaseURL, "127.0.0.1")) {
		return r.reactMock(history)
	}

	// Format conversation history for LLM review
	historyBytes, err := json.Marshal(history)
	if err != nil {
		return ReactorResponse{ModelAttention: 0.1, UserAttention: 0.7, Serotonin: 0.1, Oxytocin: 0.1, Cortisol: 0.1}, fmt.Errorf("failed to marshal history for reactor: %w", err)
	}

	systemInstruction := r.config.SystemInstruction
	if systemInstruction == "" {
		systemInstruction = prompts.GetReactorPrompt()
	}

	rawResponse, err := r.callLLM(ctx, string(historyBytes), systemInstruction)
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
		if val < -1.0 { return -1.0 }
		if val > 1.0 { return 1.0 }
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
func (r *ReactorAgent) reactMock(history []consolidator.Message) (ReactorResponse, error) {
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
				resp.Cortisol = 0.8 // Stress
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
				resp.Cortisol = 0.9 // Extreme stress
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

func (r *ReactorAgent) callLLM(ctx context.Context, prompt, systemInstruction string) (string, error) {
	if r.config.Type == "gemini" {
		return r.callGemini(ctx, prompt, systemInstruction)
	}
	return r.callOpenAI(ctx, prompt, systemInstruction)
}

func (r *ReactorAgent) callOpenAI(ctx context.Context, prompt, systemInstruction string) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", r.config.BaseURL)

	type openAIChatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	reqBody := struct {
		Model          string              `json:"model"`
		Messages       []openAIChatMessage `json:"messages"`
		ResponseFormat *struct {
			Type string `json:"type"`
		} `json:"response_format,omitempty"`
	}{
		Model: r.config.Model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemInstruction},
			{Role: "user", Content: prompt},
		},
		ResponseFormat: &struct {
			Type string `json:"type"`
		}{Type: "json_object"},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if r.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.config.APIKey))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty choices returned")
	}

	return result.Choices[0].Message.Content, nil
}

func (r *ReactorAgent) callGemini(ctx context.Context, prompt, systemInstruction string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		r.config.Model, r.config.APIKey)

	type geminiPart struct {
		Text string `json:"text"`
	}
	type geminiContent struct {
		Role  string       `json:"role,omitempty"`
		Parts []geminiPart `json:"parts"`
	}
	type geminiSystemInstruction struct {
		Parts []geminiPart `json:"parts"`
	}

	reqBody := struct {
		Contents          []geminiContent          `json:"contents"`
		SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	}{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
		SystemInstruction: &geminiSystemInstruction{
			Parts: []geminiPart{
				{Text: systemInstruction},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error *struct {
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil {
			return "", fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("Gemini returned status %d", resp.StatusCode)
	}

	var result struct {
		Candidates []struct {
			Content      geminiContent `json:"content"`
			FinishReason string        `json:"finishReason,omitempty"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Error != nil {
		return "", fmt.Errorf("Gemini API error: %s (Status: %s)", result.Error.Message, result.Error.Status)
	}

	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("empty candidates returned")
	}

	candidate := result.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		if candidate.FinishReason != "" && candidate.FinishReason != "STOP" {
			return "", fmt.Errorf("Gemini response was blocked/terminated. Reason: %s", candidate.FinishReason)
		}
		return "", fmt.Errorf("empty candidate content returned by Gemini")
	}

	return candidate.Content.Parts[0].Text, nil
}
