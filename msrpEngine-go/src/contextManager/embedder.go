package contextManager

import (
	"context"
	"math"

	embeddingsProvider "msrpengine/src/providers/embeddings"
)

// Embedder configures the connection to the embedding engine.
type Embedder struct {
	provider embeddingsProvider.Provider
}

// NewLocalEmbedder creates an embedder pointing to the configured instance.
func NewLocalEmbedder() *Embedder {
	return &Embedder{
		provider: embeddingsProvider.NewProvider(),
	}
}

// Embed generates a vector embedding for the given text by delegating to the plugin.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return e.provider.Embed(ctx, text)
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
