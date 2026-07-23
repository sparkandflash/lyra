package agents

import (
	"context"
	"fmt"

	"msrpengine/src/providers/inference"
)

// Agent represents a unified LLM client capable of communicating with various providers.
type Agent struct {
	Type         string // e.g. "gemini", "openai", "local-binary", "embedded"
	SystemPrompt string
	client       inference.Provider
}

// NewAgent creates a new Agent instance.
func NewAgent(agentType, apiKey, baseURL, model, systemPrompt string) *Agent {
	return &Agent{
		Type:         agentType,
		SystemPrompt: systemPrompt,
		client:       inference.NewProvider(agentType, apiKey, baseURL, model),
	}
}

// Generate sends the userPrompt to the configured LLM and returns the raw string response.
func (a *Agent) Generate(ctx context.Context, userPrompt string, sysPromptOverride string) (string, error) {
	activeSysPrompt := a.SystemPrompt
	if sysPromptOverride != "" {
		activeSysPrompt = sysPromptOverride
	}
	
	if a.client == nil {
		return "", fmt.Errorf("no inference provider configured for agent type: %s", a.Type)
	}

	return a.client.Generate(ctx, userPrompt, activeSysPrompt)
}

// Validate pings the provider's models endpoint to verify credentials.
func (a *Agent) Validate(ctx context.Context) error {
	if a.client == nil {
		return fmt.Errorf("no inference provider configured for agent type: %s", a.Type)
	}
	return a.client.Validate(ctx)
}
