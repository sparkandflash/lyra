package inference

import "context"

// Provider defines the interface that all inference plugins (LLMs) must implement.
type Provider interface {
	// Generate sends the userPrompt and systemPrompt to the configured LLM and returns the raw string response.
	Generate(ctx context.Context, userPrompt string, sysPrompt string) (string, error)

	// Validate pings the provider's models endpoint to verify credentials.
	Validate(ctx context.Context) error
}
