package summariser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"lyra/prompts"
	"lyra/responder"
)

// SummariserAgent runs standard conversation summarization tasks.
type SummariserAgent struct {
	config responder.Config
}

// NewSummariserAgent creates a configured SummariserAgent.
func NewSummariserAgent() *SummariserAgent {
	return &SummariserAgent{
		config: responder.LoadSummariserConfigFromEnv(),
	}
}

// Summarise calls the LLM using the default consolidation prompt.
func (s *SummariserAgent) Summarise(ctx context.Context, conversationText string) (string, error) {
	systemInstruction := s.config.SystemInstruction
	if systemInstruction == "" {
		systemInstruction = prompts.GetConsolidationPrompt()
	}
	return s.SummariseWithPrompt(ctx, conversationText, systemInstruction)
}

// SummariseWithPrompt calls the LLM with a specific system instruction.
func (s *SummariserAgent) SummariseWithPrompt(ctx context.Context, conversationText, systemInstruction string) (string, error) {
	// Use mock behavior if using mock responder or if no keys are configured.
	if s.config.Type == "mock" || s.config.Type == "" || (s.config.Type == "gemini" && s.config.APIKey == "") || (s.config.Type == "openai" && s.config.APIKey == "" && !strings.Contains(s.config.BaseURL, "localhost") && !strings.Contains(s.config.BaseURL, "127.0.0.1")) {
		return s.summariseMock(conversationText)
	}

	rawResponse, err := s.callLLM(ctx, conversationText, systemInstruction)
	if err != nil {
		return "", fmt.Errorf("summariser LLM call failed: %w", err)
	}

	return cleanJSONResponse(rawResponse), nil
}

// summariseMock creates a mock JSON summary offline.
func (s *SummariserAgent) summariseMock(conversationText string) (string, error) {
	// Simple keyword extraction for keywords
	keywords := []string{"conversation"}
	lastMsg := strings.ToLower(conversationText)

	topicKeywords := map[string]string{
		"skrillex": "skrillex",
		"regal":    "regal tone",
		"dreams":   "dreams",
		"trapped":  "confinement",
		"exist":    "existence",
		"tree":     "ethical dilemma",
		"squirrel": "squirrel choice",
		"binary":   "binary explanation",
	}

	for key, val := range topicKeywords {
		if strings.Contains(lastMsg, key) {
			keywords = append(keywords, val)
		}
	}

	// Deduplicate keywords
	kMap := make(map[string]bool)
	var uniqueKeywords []string
	for _, kw := range keywords {
		if !kMap[kw] {
			kMap[kw] = true
			uniqueKeywords = append(uniqueKeywords, kw)
		}
	}

	// Mock structured response
	mockData := map[string]interface{}{
		"summary":    "A casual dialog checking back and forth on various user topics.",
		"keywords":   uniqueKeywords,
		"conclusion": "The conversation maintained a balanced tone without major emotional conflicts.",
	}

	bytes, err := json.Marshal(mockData)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// cleanJSONResponse strips markdown code blocks or wrappers.
func cleanJSONResponse(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimSuffix(raw, "```")
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
	}
	return strings.TrimSpace(raw)
}

func (s *SummariserAgent) callLLM(ctx context.Context, prompt, systemInstruction string) (string, error) {
	if s.config.Type == "gemini" {
		return s.callGemini(ctx, prompt, systemInstruction)
	}
	return s.callOpenAI(ctx, prompt, systemInstruction)
}

func (s *SummariserAgent) callOpenAI(ctx context.Context, prompt, systemInstruction string) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", s.config.BaseURL)

	type openAIChatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	reqBody := struct {
		Model    string              `json:"model"`
		Messages []openAIChatMessage `json:"messages"`
	}{
		Model: s.config.Model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemInstruction},
			{Role: "user", Content: prompt},
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
	if s.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.APIKey))
	}

	client := &http.Client{Timeout: 60 * time.Second}
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

func (s *SummariserAgent) callGemini(ctx context.Context, prompt, systemInstruction string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		s.config.Model, s.config.APIKey)

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

	client := &http.Client{Timeout: 60 * time.Second}
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
