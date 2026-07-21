package responder

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"msrpengine/src/consolidator"
)

func TestNewResponderFromEnv(t *testing.T) {
	tests := []struct {
		name          string
		envType       string
		expectedType  string
		envApiKey     string
		expectInitErr bool
	}{
		{
			name:         "Mock Responder Default",
			envType:      "",
			expectedType: "*responder.MockResponder",
		},
		{
			name:         "Mock Responder Explicit",
			envType:      "mock",
			expectedType: "*responder.MockResponder",
		},
		{
			name:         "OpenAI Responder",
			envType:      "openai",
			expectedType: "*responder.OpenAIResponder",
		},
		{
			name:         "Gemini Responder",
			envType:      "gemini",
			expectedType: "*responder.GeminiResponder",
		},
		{
			name:         "Local Binary Responder",
			envType:      "local-binary",
			expectedType: "*responder.LocalBinaryResponder",
		},
		{
			name:          "Embedded Responder Fails with Placeholder",
			envType:       "embedded",
			expectInitErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set env
			os.Setenv("SYSTEM_RESPONDER_TYPE", tc.envType)
			defer os.Unsetenv("SYSTEM_RESPONDER_TYPE")

			resp, err := NewResponderFromEnv()
			if tc.expectInitErr {
				if err == nil {
					t.Fatalf("expected error on initialization, but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected initialization error: %v", err)
			}

			actualType := fmt.Sprintf("%T", resp)
			// Standard reflect type check using string formatting since package is internal
			if !strings.Contains(actualType, tc.expectedType) {
				t.Errorf("expected type %s, got %s", tc.expectedType, actualType)
			}
		})
	}
}

func TestMockResponder(t *testing.T) {
	config := Config{
		SystemInstruction: "test instruction",
	}
	r := NewMockResponder(config)
	history := []consolidator.Message{
		{Author: "user", Content: "prev"},
	}
	res, _, err := r.Respond(context.Background(), "hello", "0.90:0.30:0.50:0.70:0.50", history, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(res, "test instruction") {
		t.Errorf("expected output to contain system instruction, got %s", res)
	}
	if !strings.Contains(res, "0.90:0.30:0.50:0.70:0.50") {
		t.Errorf("expected output to contain mind state 0.90:0.30:0.50:0.70:0.50, got %s", res)
	}
	if !strings.Contains(res, "History Size: 1") {
		t.Errorf("expected output to contain history size 1, got %s", res)
	}
	if !strings.Contains(res, "hello") {
		t.Errorf("expected output to contain prompt, got %s", res)
	}
}

func TestLoadEnvFile(t *testing.T) {
	// Create a temporary .env file for testing
	envContent := `
# This is a comment
TEST_ENV_VAR="test-value"
TEST_ENV_VAR_SINGLE='single-value'
TEST_ENV_VAR_NO_QUOTES=raw-value
`
	err := os.WriteFile(".env", []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test .env file: %v", err)
	}
	defer os.Remove(".env")

	// Ensure they are clean in current env
	os.Unsetenv("TEST_ENV_VAR")
	os.Unsetenv("TEST_ENV_VAR_SINGLE")
	os.Unsetenv("TEST_ENV_VAR_NO_QUOTES")

	loadEnvFile()

	if val := os.Getenv("TEST_ENV_VAR"); val != "test-value" {
		t.Errorf("expected TEST_ENV_VAR to be 'test-value', got %q", val)
	}
	if val := os.Getenv("TEST_ENV_VAR_SINGLE"); val != "single-value" {
		t.Errorf("expected TEST_ENV_VAR_SINGLE to be 'single-value', got %q", val)
	}
	if val := os.Getenv("TEST_ENV_VAR_NO_QUOTES"); val != "raw-value" {
		t.Errorf("expected TEST_ENV_VAR_NO_QUOTES to be 'raw-value', got %q", val)
	}

	// Clean up env vars
	os.Unsetenv("TEST_ENV_VAR")
	os.Unsetenv("TEST_ENV_VAR_SINGLE")
	os.Unsetenv("TEST_ENV_VAR_NO_QUOTES")
}

