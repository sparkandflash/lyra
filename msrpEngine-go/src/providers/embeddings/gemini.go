package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"msrpengine/src/utils"
)

type GeminiProvider struct {
	EndpointURL string
	Model       string
	APIKey      string
}

func NewGeminiProvider(endpoint, model, apiKey string) *GeminiProvider {
	if model == "" {
		model = "text-embedding-004"
	}
	if endpoint == "" {
		endpoint = "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":embedContent"
	}
	return &GeminiProvider{
		EndpointURL: endpoint,
		Model:       model,
		APIKey:      apiKey,
	}
}

func (p *GeminiProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	url := p.EndpointURL
	if p.APIKey != "" && !strings.Contains(url, "?key=") {
		url = url + "?key=" + p.APIKey
	}

	reqBody := map[string]interface{}{
		"model": "models/" + p.Model,
		"content": map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": text},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini embedding API returned status %d", resp.StatusCode)
	}

	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	vec := result.Embedding.Values
	if len(vec) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

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

	utils.LogMetrics("embedder-gemini", len(jsonData), 0)
	return vec, nil
}
