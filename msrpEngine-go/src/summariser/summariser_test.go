package summariser

import (
	"strings"
	"testing"
)

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Clean JSON",
			input:    `{"summary": "Test Summary"}`,
			expected: `{"summary": "Test Summary"}`,
		},
		{
			name:     "Markdown Wrapped JSON",
			input:    "```json\n{\"summary\": \"Test Summary\"}\n```",
			expected: `{"summary": "Test Summary"}`,
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

func TestMockSummariser(t *testing.T) {
	agent := NewSummariserAgent()

	// Use mock behavior to test keyword extraction
	res, err := agent.summariseMock("User wants to talk about skrillex dreams.")
	if err != nil {
		t.Fatalf("unexpected mock error: %v", err)
	}

	if !strings.Contains(res, "skrillex") {
		t.Errorf("expected mock output to extract keyword 'skrillex', got: %s", res)
	}
	if !strings.Contains(res, "dreams") {
		t.Errorf("expected mock output to extract keyword 'dreams', got: %s", res)
	}
}
