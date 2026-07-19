package responder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"terminal-app/src/consolidator"
	"terminal-app/src/prompts"
)

type OpenAIResponder struct {
	config Config
}

func NewOpenAIResponder(config Config) *OpenAIResponder {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.cerebras.ai/v1" // Fallback to Cerebras default
	}
	if config.Model == "" {
		config.Model = "gpt-oss-120b" // Fallback to a supported Cerebras default
	}
	return &OpenAIResponder{config: config}
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatRequest struct {
	Model    string              `json:"model"`
	Messages []openAIChatMessage `json:"messages"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (r *OpenAIResponder) Respond(ctx context.Context, prompt string, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (string, string, error) {
	systemPrompt := prompts.GetResponderPrompt()
	if r.config.SystemInstruction != "" {
		systemPrompt = r.config.SystemInstruction
	}
	return r.respondInternal(ctx, prompt, mindState, history, episodes, systemPrompt)
}

func (r *OpenAIResponder) RespondProactive(ctx context.Context, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (string, string, error) {
	systemPrompt := prompts.GetProactivePrompt()
	return r.respondInternal(ctx, "[System: The user has been silent. Initiate conversation.]", mindState, history, episodes, systemPrompt)
}

func (r *OpenAIResponder) respondInternal(ctx context.Context, prompt string, mindState string, history []consolidator.Message, episodes []EpisodeSummary, systemPrompt string) (string, string, error) {
	url := fmt.Sprintf("%s/chat/completions", r.config.BaseURL)

	messages := []openAIChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// Format context and user input as readable text so small models don't get confused by raw JSON
	var promptBuilder bytes.Buffer

	// Extract optional energy hint appended as "mindstate|energy:N"
	actualMindState := mindState
	energyLabel := ""
	if idx := strings.Index(mindState, "|energy:"); idx != -1 {
		actualMindState = mindState[:idx]
		energyLabel = mindState[idx+8:] // start after "|energy:"
		if sepIdx := strings.Index(energyLabel, "|"); sepIdx != -1 {
			energyLabel = energyLabel[:sepIdx]
		}
	}

	promptBuilder.WriteString(fmt.Sprintf("[Current Mindstate: %s]\n", actualMindState))
	if energyLabel != "" {
		promptBuilder.WriteString(fmt.Sprintf("[Mental Energy: %s/1000]\n", energyLabel))
	}
	
	if len(episodes) > 0 {
		promptBuilder.WriteString("\n[Recalled Episodic Memories]\n")
		for _, ep := range episodes {
			promptBuilder.WriteString(fmt.Sprintf("- Episode %s: %s\n", ep.ID, strings.Join(ep.Facts, " ")))
		}
	}

	promptBuilder.WriteString("\n[User Message]\n")
	promptBuilder.WriteString(prompt)

	messages = append(messages, openAIChatMessage{
		Role:    "user",
		Content: promptBuilder.String(),
	})

	reqBody := openAIChatRequest{
		Model:    r.config.Model,
		Messages: messages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if r.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.config.APIKey))
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr openAIChatResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil {
			return "", "", fmt.Errorf("API returned non-200 status: %d - %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", "", fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", "", fmt.Errorf("no response choices returned by model")
	}

	rawContent := chatResp.Choices[0].Message.Content
	return parseResponderOutput(rawContent)
}
