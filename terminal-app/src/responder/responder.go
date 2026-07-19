package responder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"terminal-app/src/consolidator"
	"terminal-app/src/prompts"
)

// EpisodeSummary is a lightweight episode view passed to the responder LLM as episodic context.
type EpisodeSummary struct {
	ID            string   `json:"id"`
	Facts         []string `json:"facts"`
	PeakMindState string   `json:"peak_mindstate"`
}

// Responder defines the interface for generating responses from LLMs.
// It returns:
//   - reply:           the conversational reply text to show the user
//   - usefulEpisodeID: the episode ID the model found most relevant (empty string if none)
//   - err:             any error that occurred
type Responder interface {
	Respond(ctx context.Context, prompt string, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (reply string, usefulEpisodeID string, err error)
	RespondProactive(ctx context.Context, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (reply string, usefulEpisodeID string, err error)
}

// parseResponderOutput parses the structured JSON the responder LLM is expected to return.
// If the raw content is not valid JSON (e.g. legacy plain-text mode or mock), the raw
// content is returned as the reply with an empty usefulEpisodeID.
func parseResponderOutput(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	// Strip markdown code fences if present
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}

	var out struct {
		Reply           string `json:"reply"`
		UsefulEpisodeID string `json:"useful_episode_id"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		// Graceful fallback: treat the whole response as plain reply text
		return raw, "", nil
	}
	return out.Reply, out.UsefulEpisodeID, nil
}

// Config holds the configuration for responders loaded from environment variables.
type Config struct {
	Type              string // gemini, openai, local-binary, embedded, mock
	APIKey            string
	BaseURL           string
	Model             string
	LocalBinaryPath   string
	SystemInstruction string
	ReactorCharThreshold int
}

func loadAgentConfig(prefix string, defaultPrompt string) Config {

	getType := func() string {
		if val := os.Getenv("SYSTEM_" + prefix + "_TYPE"); val != "" {
			return val
		}
		return os.Getenv("SYSTEM_RESPONDER_TYPE") // Base fallback for type (legacy support)
	}

	getVar := func(name string, fallback string) string {
		if val := os.Getenv("SYSTEM_" + prefix + "_" + name); val != "" {
			return val
		}
		return os.Getenv(fallback)
	}

	sysInst := getVar("SYSTEM_INSTRUCTION", "SYSTEM_SYSTEM_INSTRUCTION")
	if sysInst == "" {
		sysInst = defaultPrompt
	}

	return Config{
		Type:              strings.ToLower(strings.TrimSpace(getType())),
		APIKey:            getVar("API_KEY", "SYSTEM_API_KEY"),
		BaseURL:           getVar("BASE_URL", "SYSTEM_BASE_URL"),
		Model:             getVar("MODEL", "SYSTEM_MODEL"),
		LocalBinaryPath:   getVar("LOCAL_BINARY_PATH", "SYSTEM_LOCAL_BINARY_PATH"),
		SystemInstruction: sysInst,
	}
}

// LoadConfigFromEnv reads configurations from environment variables.
func LoadConfigFromEnv() Config {
	return loadAgentConfig("RESPONDER", prompts.GetResponderPrompt())
}

// LoadReactorConfigFromEnv reads reactor-specific configurations from environment variables.
func LoadReactorConfigFromEnv() Config {
	cfg := loadAgentConfig("REACTOR", prompts.GetReactorPrompt())
	
	thresholdStr := os.Getenv("SYSTEM_REACTOR_CHAR_THRESHOLD")
	if thresholdStr == "" {
		cfg.ReactorCharThreshold = 600
	} else {
		var err error
		if _, err = fmt.Sscanf(thresholdStr, "%d", &cfg.ReactorCharThreshold); err != nil {
			cfg.ReactorCharThreshold = 600
		}
	}
	return cfg
}

// LoadSummariserConfigFromEnv reads summariser-specific configurations from environment variables.
func LoadSummariserConfigFromEnv() Config {
	return loadAgentConfig("SUMMARISER", prompts.GetConsolidationPrompt())
}

// ValidateConfig pings the provider's /models endpoint to verify credentials.
func ValidateConfig(ctx context.Context, cfg Config) error {
	if cfg.Type == "mock" || cfg.Type == "embedded" || cfg.Type == "local-binary" || cfg.Type == "" {
		return nil // skip validation for local/mock
	}

	var url string
	var req *http.Request
	var err error

	if cfg.Type == "gemini" {
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", cfg.APIKey)
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
	} else if cfg.Type == "openai" {
		if cfg.BaseURL == "" {
			return fmt.Errorf("missing base URL")
		}
		url = fmt.Sprintf("%s/models", strings.TrimSuffix(cfg.BaseURL, "/"))
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err == nil && cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
	} else {
		return fmt.Errorf("unknown config type %q", cfg.Type)
	}

	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code %d from %s", resp.StatusCode, url)
	}
	return nil
}

// loadEnvFile parses the local .env file and sets environment variables if they are not already set.
func loadEnvFile() {
	file, err := os.Open(".env")
	if err != nil {
		exePath, errExe := os.Executable()
		if errExe != nil {
			return // .env file is optional
		}
		exeDir := filepath.Dir(exePath)
		file, err = os.Open(filepath.Join(exeDir, ".env"))
		if err != nil {
			return // .env file is optional
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Strip quotes if they surround the value
		val = strings.Trim(val, `"'`)

		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

// NewResponderFromEnv initializes a responder based on the environment config.
func NewResponderFromEnv() (Responder, error) {
	config := LoadConfigFromEnv()
	if config.SystemInstruction == "" {
		config.SystemInstruction = prompts.GetResponderPrompt()
	}
	if config.Type == "" {
		config.Type = "mock"
	}

	switch config.Type {
	case "gemini":
		return NewGeminiResponder(config), nil
	case "openai":
		return NewOpenAIResponder(config), nil
	case "local-binary":
		return NewLocalBinaryResponder(config), nil
	case "embedded":
		return NewEmbeddedResponder(config)
	case "mock":
		fallthrough
	default:
		return NewMockResponder(config), nil
	}
}
