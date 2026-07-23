package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"msrpengine/src/utils"
)

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float32         `json:"temperature,omitempty"`
}
type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type OpenAIProvider struct {
	BaseURL string
	APIKey  string
	Model   string
}

func NewOpenAIProvider(baseURL, apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	}
}

func (p *OpenAIProvider) Validate(ctx context.Context) error {
	if p.BaseURL == "" {
		return fmt.Errorf("missing base URL for OpenAI provider")
	}

	url := fmt.Sprintf("%s/models", strings.TrimSuffix(p.BaseURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code %d from %s", resp.StatusCode, url)
	}
	return nil
}

func (p *OpenAIProvider) Generate(ctx context.Context, userPrompt string, activeSysPrompt string) (string, error) {
	if p.APIKey == "" {
		return "", fmt.Errorf("openai api key is required")
	}
	if p.BaseURL == "" {
		return "", fmt.Errorf("missing base URL for OpenAI provider")
	}

	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(p.BaseURL, "/"))

	reqBody := openAIRequest{
		Model:       p.Model,
		Temperature: 0.7,
		Messages:    []openAIMessage{},
	}

	if activeSysPrompt != "" {
		reqBody.Messages = append(reqBody.Messages, openAIMessage{Role: "system", Content: activeSysPrompt})
	}
	reqBody.Messages = append(reqBody.Messages, openAIMessage{Role: "user", Content: userPrompt})

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr openAIResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil {
			return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("OpenAI API returned non-200 status: %d", resp.StatusCode)
	}

	var oaiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&oaiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned by OpenAI")
	}

	result := oaiResp.Choices[0].Message.Content
	utils.LogMetrics("openai", len(jsonData), len(result))
	return result, nil
}
