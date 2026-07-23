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

type CohereProvider struct {
	EndpointURL string
	Model       string
	APIKey      string
}

func NewCohereProvider(endpoint, model, apiKey string) *CohereProvider {
	if model == "" {
		model = "embed-english-v3.0"
	}
	if endpoint == "" {
		endpoint = "https://api.cohere.com/v1/embed"
	}
	return &CohereProvider{
		EndpointURL: endpoint,
		Model:       model,
		APIKey:      apiKey,
	}
}

func (p *CohereProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model":      p.Model,
		"texts":      []string{text},
		"input_type": "search_document",
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
		return nil, fmt.Errorf("cohere embedding API returned status %d", resp.StatusCode)
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

	utils.LogMetrics("embedder-cohere", len(jsonData), 0)
	return vec, nil
}
