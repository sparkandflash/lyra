package embeddings

import (
	"os"
	"strings"
)

// NewProvider creates a new embedder Provider based on the EMBEDDING_PROVIDER environment variable.
// If the variable is not set, it defaults to "openai" (which is also compatible with local engines like Ollama).
func NewProvider() Provider {
	provider := strings.ToLower(os.Getenv("EMBEDDING_PROVIDER"))
	
	endpoint := os.Getenv("EMBEDDING_API_URL")
	model := os.Getenv("EMBEDDING_MODEL")
	apiKey := os.Getenv("EMBEDDING_API_KEY")

	// Fallback heuristic if EMBEDDING_PROVIDER is not set explicitly
	if provider == "" {
		if strings.Contains(endpoint, "cohere.com") {
			provider = "cohere"
		} else if strings.Contains(endpoint, "generativelanguage.googleapis.com") {
			provider = "gemini"
		} else {
			provider = "openai" // Default / Fallback
		}
	}

	switch provider {
	case "gemini":
		return NewGeminiProvider(endpoint, model, apiKey)
	case "cohere":
		return NewCohereProvider(endpoint, model, apiKey)
	case "openai":
		fallthrough
	default:
		return NewOpenAIProvider(endpoint, model, apiKey)
	}
}
