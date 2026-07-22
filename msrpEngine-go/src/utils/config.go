package utils

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// AppConfig holds all environment variables in a strongly-typed struct.
type AppConfig struct {
	// Base Configuration
	SystemAPIKey          string
	SystemBaseURL         string
	SystemModel           string
	SystemPersonalityName string

	// Embedding
	EmbeddingAPIURL string
	EmbeddingModel  string
	EmbeddingAPIKey string

	// Responder
	ResponderType     string
	ResponderAPIKey   string
	ResponderBaseURL  string
	ResponderModel    string
	ResponderSTMChars int

	// Reactor
	ReactorType          string
	ReactorAPIKey        string
	ReactorBaseURL       string
	ReactorModel         string
	ReactorCharThreshold int

	// Summariser
	SummariserType    string
	SummariserAPIKey  string
	SummariserBaseURL string
	SummariserModel   string

	// Engine & Memory Settings
	TempSleepCycleMins    int
	MaxWorkingMemoryChars int
	EpisodeMemoryChars    int
	TickSeconds           int
	ConsolidationDensity  int
	ConsolidationFreqMins int
	TrueSleepDelayMins    int
	TempSleepDelayMins    int

	// Character Limits
	SystemPromptCharLimit  int
	ResponderMasterLimit   int
	ReactorMasterLimit     int
	SummariserMasterLimit  int
	EmbeddingMaxInputLimit int
	SystemMaxInputChars    int
	SystemMaxOutputChars   int

	// Web API
	Port      int
	WebUser   string
	WebPass   string
	JWTSecret string
}

// Config is the global configuration object.
// It is automatically populated on startup.
var Config *AppConfig

func init() {
	// Attempt to load .env using the resolved path.
	// This ensures it works whether we run via 'go run', built binary, or tests.
	envPath := ResolvePath(".env")
	if err := godotenv.Load(envPath); err != nil {
		log.Printf("Info: No .env file found at %s or error loading it. Relying on system environment variables.", envPath)
	}

	Config = &AppConfig{
		SystemAPIKey:          getEnv("SYSTEM_API_KEY", ""),
		SystemBaseURL:         getEnv("SYSTEM_BASE_URL", ""),
		SystemModel:           getEnv("SYSTEM_MODEL", ""),
		SystemPersonalityName: getEnv("SYSTEM_PERSONALITY_NAME", ""),

		EmbeddingAPIURL: getEnv("EMBEDDING_API_URL", ""),
		EmbeddingModel:  getEnv("EMBEDDING_MODEL", ""),
		EmbeddingAPIKey: getEnv("EMBEDDING_API_KEY", ""),

		ResponderType:     getEnv("SYSTEM_RESPONDER_TYPE", ""),
		ResponderAPIKey:   getEnv("SYSTEM_RESPONDER_API_KEY", ""),
		ResponderBaseURL:  getEnv("SYSTEM_RESPONDER_BASE_URL", ""),
		ResponderModel:    getEnv("SYSTEM_RESPONDER_MODEL", ""),
		ResponderSTMChars: getEnvAsInt("SYSTEM_RESPONDER_STM_CHARS", 4000),

		ReactorType:          getEnv("SYSTEM_REACTOR_TYPE", ""),
		ReactorAPIKey:        getEnv("SYSTEM_REACTOR_API_KEY", ""),
		ReactorBaseURL:       getEnv("SYSTEM_REACTOR_BASE_URL", ""),
		ReactorModel:         getEnv("SYSTEM_REACTOR_MODEL", ""),
		ReactorCharThreshold: getEnvAsInt("SYSTEM_REACTOR_CHAR_THRESHOLD", 600),

		SummariserType:    getEnv("SYSTEM_SUMMARISER_TYPE", ""),
		SummariserAPIKey:  getEnv("SYSTEM_SUMMARISER_API_KEY", ""),
		SummariserBaseURL: getEnv("SYSTEM_SUMMARISER_BASE_URL", ""),
		SummariserModel:   getEnv("SYSTEM_SUMMARISER_MODEL", ""),

		TempSleepCycleMins:    getEnvAsInt("SYSTEM_TEMP_SLEEP_CYCLE_MINS", 60),
		MaxWorkingMemoryChars: getEnvAsInt("SYSTEM_MAX_WORKING_MEMORY_CHARS", 3000),
		EpisodeMemoryChars:    getEnvAsInt("SYSTEM_EPISODE_MEMORY_CHARS", 8000),
		TickSeconds:           getEnvAsInt("SYSTEM_TICK_SECONDS", 5),
		ConsolidationDensity:  getEnvAsInt("SYSTEM_CONSOLIDATION_DENSITY", 3000),
		ConsolidationFreqMins: getEnvAsInt("SYSTEM_CONSOLIDATION_FREQ_MINS", 1),
		TrueSleepDelayMins:    getEnvAsInt("SYSTEM_TRUE_SLEEP_DELAY_MINS", 180),
		TempSleepDelayMins:    getEnvAsInt("SYSTEM_TEMP_SLEEP_DELAY_MINS", 5),

		SystemPromptCharLimit:  getEnvAsInt("SYSTEM_PROMPT_CHAR_LIMIT", 1000),
		ResponderMasterLimit:   getEnvAsInt("SYSTEM_RESPONDER_MASTER_LIMIT", 16000),
		ReactorMasterLimit:     getEnvAsInt("SYSTEM_REACTOR_MASTER_LIMIT", 5000),
		SummariserMasterLimit:  getEnvAsInt("SYSTEM_SUMMARISER_MASTER_LIMIT", 16000),
		EmbeddingMaxInputLimit: getEnvAsInt("EMBEDDING_MAX_INPUT_LIMIT", 32000),
		SystemMaxInputChars:    getEnvAsInt("SYSTEM_MAX_INPUT_CHARS", 200),
		SystemMaxOutputChars:   getEnvAsInt("SYSTEM_MAX_OUTPUT_CHARS", 200),

		Port:      getEnvAsInt("PORT", 8080),
		WebUser:   getEnv("WEB_USER", ""),
		WebPass:   getEnv("WEB_PASS", ""),
		JWTSecret: getEnv("JWT_SECRET", ""),
	}

	if err := Config.ValidateLimits(); err != nil {
		log.Fatalf("Fatal configuration error: %v", err)
	}
}

// ValidateLimits ensures the context windows do not exceed the configured LLM master limits.
func (c *AppConfig) ValidateLimits() error {
	// A. Responder Limits
	responderTotal := c.ResponderSTMChars + c.EpisodeMemoryChars + c.SystemPromptCharLimit
	if responderTotal > c.ResponderMasterLimit {
		return fmt.Errorf("responder sub-limits (%d) exceed ResponderMasterLimit (%d)", responderTotal, c.ResponderMasterLimit)
	}

	// B. Reactor Limits
	reactorTotal := c.MaxWorkingMemoryChars + c.SystemPromptCharLimit
	if reactorTotal > c.ReactorMasterLimit {
		return fmt.Errorf("reactor sub-limits (%d) exceed ReactorMasterLimit (%d)", reactorTotal, c.ReactorMasterLimit)
	}
	if c.ReactorCharThreshold > (c.ReactorMasterLimit - c.SystemPromptCharLimit) {
		return fmt.Errorf("reactor threshold (%d) exceeds available budget (%d)", c.ReactorCharThreshold, c.ReactorMasterLimit-c.SystemPromptCharLimit)
	}

	// C. & D. Summariser and Embedding Limits
	// (Assuming the consolidation density represents an idle method's max input limit)
	if c.ConsolidationDensity > c.SummariserMasterLimit {
		return fmt.Errorf("idle method limit (%d) exceeds SummariserMasterLimit (%d)", c.ConsolidationDensity, c.SummariserMasterLimit)
	}
	if c.SummariserMasterLimit > c.EmbeddingMaxInputLimit {
		return fmt.Errorf("summariser master limit (%d) exceeds EmbeddingMaxInputLimit (%d)", c.SummariserMasterLimit, c.EmbeddingMaxInputLimit)
	}

	return nil
}

// getEnv fetches an environment variable or returns a fallback string.
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// getEnvAsInt fetches an environment variable and parses it as an int.
func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if value, err := strconv.Atoi(valStr); err == nil {
		return value
	}
	return defaultVal
}
