package inference

// NewProvider creates a new inference Provider based on the requested agent type.
func NewProvider(agentType, apiKey, baseURL, model string) Provider {
	switch agentType {
	case "gemini":
		return NewGeminiProvider(apiKey, model)
	case "openai":
		return NewOpenAIProvider(baseURL, apiKey, model)
	case "mock":
		return NewMockProvider()
	default:
		// Fallback to OpenAI compatible for unrecognized strings
		return NewOpenAIProvider(baseURL, apiKey, model)
	}
}
