package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"msrpengine/src/utils"
)

type OpenAIProvider struct {
	EndpointURL string
	Model       string
	APIKey      string
}

func NewOpenAIProvider(endpoint, model, apiKey string) *OpenAIProvider {
	if model == "" {
		model = "nomic-embed-text"
	}
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11435/api/embed"
	}
	return &OpenAIProvider{
		EndpointURL: endpoint,
		Model:       model,
		APIKey:      apiKey,
	}
}

func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model": p.Model,
		"input": text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.EndpointURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai embedding API returned status %d", resp.StatusCode)
	}

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	vec := result.Embeddings[0]

	// Normalize the vector
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	utils.LogMetrics("embedder-openai", len(jsonData), 0)
	return vec, nil
}
