package responder

import (
	"context"
	"fmt"

	"msrpengine/src/consolidator"
	"msrpengine/src/prompts"
)

type MockResponder struct {
	config Config
}

func NewMockResponder(config Config) *MockResponder {
	return &MockResponder{config: config}
}

func (r *MockResponder) Respond(ctx context.Context, prompt string, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (string, string, error) {
	systemPrompt := prompts.GetResponderPrompt()
	if r.config.SystemInstruction != "" {
		systemPrompt = r.config.SystemInstruction
	}
	reply := fmt.Sprintf("[Mock Response] (System Instruction: %q, Mind State: %q, History Size: %d, Episodes: %d) You said: %s", systemPrompt, mindState, len(history), len(episodes), prompt)
	return reply, "", nil
}

func (r *MockResponder) RespondProactive(ctx context.Context, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (string, string, error) {
	systemPrompt := prompts.GetProactivePrompt()
	reply := fmt.Sprintf("[Mock Proactive] (System Instruction: %q, Mind State: %q) Initiating conversation.", systemPrompt, mindState)
	return reply, "", nil
}
