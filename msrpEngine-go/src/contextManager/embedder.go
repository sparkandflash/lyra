package contextManager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// Embedder configures the connection to the embedding engine.
type Embedder struct {
	EndpointURL string
	Model       string
	APIKey      string
}

// NewLocalEmbedder creates an embedder pointing to the configured instance.
func NewLocalEmbedder() *Embedder {
	endpoint := os.Getenv("EMBEDDING_API_URL")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11435/api/embed"
	}
	model := os.Getenv("EMBEDDING_MODEL")
	if model == "" {
		model = "nomic-embed-text"
	}
	apiKey := os.Getenv("EMBEDDING_API_KEY")
	return &Embedder{
		EndpointURL: endpoint,
		Model:       model,
		APIKey:      apiKey,
	}
}

// Embed generates a vector embedding for the given text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	url := e.EndpointURL
	
	var reqBody interface{}
	// Simple heuristic: if URL contains cohere, use Cohere's payload format
	if strings.Contains(url, "cohere.com") {
		reqBody = map[string]interface{}{
			"model": e.Model,
			"texts": []string{text},
			"input_type": "search_document",
		}
	} else {
		reqBody = map[string]interface{}{
			"model": e.Model,
			"input": text,
		}
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
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer " + e.APIKey)
	}

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

	vec := result.Embeddings[0]
	// Normalize the vector for chromem-go
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

	return vec, nil
}

// AsChromemEmbeddingFunc returns a chromem.EmbeddingFunc for use with chromem-go.
func (e *Embedder) AsChromemEmbeddingFunc() func(ctx context.Context, text string) ([]float32, error) {
	return func(ctx context.Context, text string) ([]float32, error) {
		return e.Embed(ctx, text)
	}
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
