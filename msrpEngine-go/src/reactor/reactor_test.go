package reactor

import (
	"context"
	"testing"

	"msrpengine/src/consolidator"
)

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Clean JSON",
			input:    `{"direction": "increase", "magnitude": 0.05}`,
			expected: `{"direction": "increase", "magnitude": 0.05}`,
		},
		{
			name:     "Markdown Wrapped JSON",
			input:    "```json\n{\"direction\": \"decrease\", \"magnitude\": 0.10}\n```",
			expected: `{"direction": "decrease", "magnitude": 0.10}`,
		},
		{
			name:     "Generic Markdown Block",
			input:    "```\n{\"direction\": \"stable\", \"magnitude\": 0.0}\n```",
			expected: `{"direction": "stable", "magnitude": 0.0}`,
		},
		{
			name:     "Spaced Markdown Wrapping",
			input:    "   ```json\n{\n  \"direction\": \"increase\"\n}\n```   ",
			expected: "{\n  \"direction\": \"increase\"\n}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := cleanJSONResponse(tc.input)
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestMockReactorEscalation(t *testing.T) {
	agent := NewReactorAgent()

	// 1. Test positive input
	historyPositive := []consolidator.Message{
		{Author: "user", Content: "I am so excited and happy right now!"},
		{Author: "assistant", Content: "That is wonderful!"},
	}
	resp, err := agent.React(context.Background(), historyPositive)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Serotonin <= 0.5 {
		t.Errorf("expected serotonin to increase, got %.2f", resp.Serotonin)
	}

	// 2. Test negative input
	historyNegative := []consolidator.Message{
		{Author: "user", Content: "I absolutely despise this terrible situation and I hate it!"},
		{Author: "assistant", Content: "I'm so sorry you feel that way."},
	}
	resp, err = agent.React(context.Background(), historyNegative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Cortisol <= 0.3 {
		t.Errorf("expected cortisol to increase, got %.2f", resp.Cortisol)
	}

	// 3. Test stable/default input
	historyStable := []consolidator.Message{
		{Author: "user", Content: "Standard text."},
	}
	resp, err = agent.React(context.Background(), historyStable)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Cortisol > 0.4 || resp.Serotonin > 0.4 {
		t.Errorf("expected emotions to stay near default baseline, got %+v", resp)
	}
}
