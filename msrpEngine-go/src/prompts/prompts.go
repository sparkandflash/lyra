package prompts

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
)

//go:embed responder.txt
var rawResponderPrompt string

//go:embed reactor.txt
var rawReactorPrompt string

//go:embed personality.txt
var rawPersonalityPrompt string

//go:embed consolidation.txt
var rawConsolidationPrompt string

func injectPersonalityName(prompt string) string {
	name := os.Getenv("SYSTEM_PERSONALITY_NAME")
	if name == "" {
		name = "Lyra"
	}
	return strings.ReplaceAll(prompt, "{{PERSONALITY_NAME}}", name)
}

// GetResponderPrompt returns the responder prompt combined with the personality prompt if defined.
func GetResponderPrompt() string {
	pers := strings.TrimSpace(rawPersonalityPrompt)
	base := injectPersonalityName(strings.TrimSpace(rawResponderPrompt))
	if pers == "" {
		return base
	}
	return fmt.Sprintf("%s\n\nPersonality guidelines:\n%s", base, pers)
}

// GetReactorPrompt returns the reactor prompt combined with the personality prompt if defined.
func GetReactorPrompt() string {
	pers := strings.TrimSpace(rawPersonalityPrompt)
	base := injectPersonalityName(strings.TrimSpace(rawReactorPrompt))
	if pers == "" {
		return base
	}
	return fmt.Sprintf("%s\n\nPersonality guidelines:\n%s", base, pers)
}

// GetConsolidationPrompt returns the consolidation base prompt.
func GetConsolidationPrompt() string {
	return injectPersonalityName(strings.TrimSpace(rawConsolidationPrompt))
}

//go:embed introspection.txt
var rawIntrospectionPrompt string

// GetIntrospectionPrompt returns the introspection base prompt combined with the personality prompt if defined.
func GetIntrospectionPrompt() string {
	pers := strings.TrimSpace(rawPersonalityPrompt)
	base := injectPersonalityName(strings.TrimSpace(rawIntrospectionPrompt))
	if pers == "" {
		return base
	}
	return fmt.Sprintf("%s\n\nPersonality guidelines:\n%s", base, pers)
}

//go:embed proactive_message.txt
var rawProactivePrompt string

// GetProactivePrompt returns the proactive message prompt combined with the personality prompt if defined.
func GetProactivePrompt() string {
	pers := strings.TrimSpace(rawPersonalityPrompt)
	base := injectPersonalityName(strings.TrimSpace(rawProactivePrompt))
	if pers == "" {
		return base
	}
	return fmt.Sprintf("%s\n\nPersonality guidelines:\n%s", base, pers)
}
