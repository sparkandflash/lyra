package inference

import (
	"context"
	"fmt"
)

type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) Validate(ctx context.Context) error {
	// Mock always validates successfully
	return nil
}

func (p *MockProvider) Generate(ctx context.Context, userPrompt string, activeSysPrompt string) (string, error) {
	// Returns a fixed structure indicating it's a mock response
	return fmt.Sprintf(`{"reply":"Mock response to: %s","useful_episode_id":""}`, userPrompt), nil
}
