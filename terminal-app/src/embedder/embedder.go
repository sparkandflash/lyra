package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

// Embedder handles vector embeddings via the local Ollama sidecar.
type Embedder struct {
	BaseURL string
	Model   string
}

// NewLocalEmbedder creates an embedder pointing to the local sidecar Ollama.
func NewLocalEmbedder() *Embedder {
	return &Embedder{
		BaseURL: "http://127.0.0.1:11435",
		Model:   "nomic-embed-text",
	}
}

// Embed generates a vector embedding for the given text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf("%s/api/embed", e.BaseURL)
	
	reqBody := struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}{
		Model: e.Model,
		Input: text,
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
		return nil, fmt.Errorf("ollama API returned status %d", resp.StatusCode)
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

	return result.Embeddings[0], nil
}

// CosineSimilarity calculates the semantic similarity between two embedding vectors.
func CosineSimilarity(a, b []float32) float64 {
	var dotProduct, normA, normB float64
	for i := 0; i < len(a) && i < len(b); i++ {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
