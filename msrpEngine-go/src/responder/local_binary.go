package responder

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"terminal-app/src/consolidator"
	"terminal-app/src/prompts"
)

type LocalBinaryResponder struct {
	config Config
}

func NewLocalBinaryResponder(config Config) *LocalBinaryResponder {
	if config.LocalBinaryPath == "" {
		config.LocalBinaryPath = "llama-cli" // Default fallback binary name
	}
	if config.Model == "" {
		config.Model = "./models/default.gguf" // Default model path fallback
	}
	return &LocalBinaryResponder{config: config}
}

func (r *LocalBinaryResponder) Respond(ctx context.Context, prompt string, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (string, string, error) {
	systemPrompt := prompts.GetResponderPrompt()
	if r.config.SystemInstruction != "" {
		systemPrompt = r.config.SystemInstruction
	}
	return r.respondInternal(ctx, prompt, mindState, history, episodes, systemPrompt)
}

func (r *LocalBinaryResponder) RespondProactive(ctx context.Context, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (string, string, error) {
	systemPrompt := prompts.GetProactivePrompt()
	return r.respondInternal(ctx, "[System: The user has been silent. Initiate conversation.]", mindState, history, episodes, systemPrompt)
}

func (r *LocalBinaryResponder) respondInternal(ctx context.Context, prompt string, mindState string, history []consolidator.Message, episodes []EpisodeSummary, systemPrompt string) (string, string, error) {
	// Construct the JSON payload for the prompt
	userPayload := map[string]interface{}{
		"message":   prompt,
		"mindstate": mindState,
		"history":   history,
		"episodes":  episodes,
	}
	payloadBytes, err := json.Marshal(userPayload)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal user payload: %w", err)
	}
	jsonPrompt := string(payloadBytes)

	// For llama-cli: -m <model> -p "<prompt>"
	// We format it with a standard chat template, embedding the system prompt and the JSON user prompt
	fullPrompt := fmt.Sprintf("<|system|>\n%s\n<|user|>\n%s\n<|assistant|>\n", systemPrompt, jsonPrompt)

	args := []string{
		"-m", r.config.Model,
		"-p", fullPrompt,
		"--n-predict", "256", // Limit tokens to keep execution fast on CPU
		"--log-disable",      // Disable llama-cli logging output
	}

	cmd := exec.CommandContext(ctx, r.config.LocalBinaryPath, args...)

	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return "", "", fmt.Errorf("local binary execution failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	output := stdout.String()
	output = strings.TrimPrefix(output, fullPrompt)
	return parseResponderOutput(strings.TrimSpace(output))
}
