package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"msrpengine/src/utils"
)

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
type geminiGenerateRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
}
type geminiGenerateResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason,omitempty"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

type GeminiProvider struct {
	APIKey string
	Model  string
}

func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	return &GeminiProvider{
		APIKey: apiKey,
		Model:  model,
	}
}

func (p *GeminiProvider) Validate(ctx context.Context) error {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", p.APIKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
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

func (p *GeminiProvider) Generate(ctx context.Context, userPrompt string, activeSysPrompt string) (string, error) {
	if p.APIKey == "" {
		return "", fmt.Errorf("gemini api key is required")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.Model, p.APIKey)

	reqBody := geminiGenerateRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: userPrompt},
				},
			},
		},
	}

	if activeSysPrompt != "" {
		reqBody.SystemInstruction = &geminiSystemInstruction{
			Parts: []geminiPart{{Text: activeSysPrompt}},
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr geminiGenerateResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil {
			return "", fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("Gemini API returned non-200 status: %d", resp.StatusCode)
	}

	var geminiResp geminiGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return "", fmt.Errorf("no response candidates returned by Gemini")
	}

	candidate := geminiResp.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		if candidate.FinishReason != "" && candidate.FinishReason != "STOP" {
			return "", fmt.Errorf("Gemini response blocked/terminated. Reason: %s", candidate.FinishReason)
		}
		return "", fmt.Errorf("no response candidate content returned")
	}

	result := candidate.Content.Parts[0].Text
	utils.LogMetrics("gemini", len(jsonData), len(result))
	return result, nil
}
