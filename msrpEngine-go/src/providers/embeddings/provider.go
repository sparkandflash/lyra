package embeddings

import "context"

// Provider defines the interface that all embedding plugins must implement.
type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
