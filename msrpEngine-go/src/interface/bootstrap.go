package engineInterface

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"msrpengine/src/agents/reactor"
	"msrpengine/src/agents/responder"
	"msrpengine/src/contextManager"
	"msrpengine/src/escalator"
	"msrpengine/src/idle_methods/episode_memory"
	"msrpengine/src/utils"
)

// NewAppCore creates and initializes the application core.
func NewAppCore(newSession bool, reuseSession string, debugMode bool) (*AppCore, error) {
	personalityName := utils.Config.SystemPersonalityName
	if personalityName == "" {
		personalityName = "simulation" // default fallback
	}

	// Start Ollama sidecar only if no remote URL is provided
	var ollamaCmd *exec.Cmd
	if utils.Config.EmbeddingAPIURL == "" {
		binDir := utils.ResolvePath(".bin")
		ollamaPath := filepath.Join(binDir, "ollama")
		if _, err := os.Stat(ollamaPath); os.IsNotExist(err) {
			fmt.Println("system error: ollama bin and embedding model files are missing.")
			os.Exit(1)
		}

		modelsDir := filepath.Join(binDir, "models")
		ollamaCmd = exec.Command(ollamaPath, "serve")
		ollamaCmd.Env = append(os.Environ(),
			fmt.Sprintf("OLLAMA_MODELS=%s", modelsDir),
			"OLLAMA_HOST=127.0.0.1:11435",
		)

		if err := ollamaCmd.Start(); err != nil {
			return nil, fmt.Errorf("system error: failed to start local embedding engine: %v", err)
		}
	}

	// Initialize the responder agent from environment configuration
	resp, err := responder.NewResponderFromEnv()
	if err != nil {
		return nil, fmt.Errorf("system error: failed to initialize responder: %v", err)
	}

	indexMgr, err := contextManager.NewChromemIndexManager()
	if err != nil {
		return nil, fmt.Errorf("failed to init index manager: %w", err)
	}

	// Initialize the reactor agent and its threshold
	reactorAgent := reactor.NewReactorAgent()

	// ── Reactor STM ──────────────────────────────────────────────────────────
	reactorMaxChars := utils.Config.MaxWorkingMemoryChars
	reactorSTM := contextManager.NewShortTermContext(reactorMaxChars)

	// ── Responder STM ────────────────────────────────────────────────────────
	responderMaxChars := utils.Config.ResponderSTMChars
	//note: interfacememory(stm) initialsed for responder.
	responderSTM := contextManager.NewShortTermContext(responderMaxChars)

	episodeMgr := episode_memory.LoadEpisodeMemoryManagerFromEnv()

	maxInputChars := utils.Config.SystemMaxInputChars
	maxOutputChars := utils.Config.SystemMaxOutputChars

	// ── Session Resolution ───────────────────────────────────────────────────
	sessionID, savedMindState, savedMentalEnergy := contextManager.ResolveSession(newSession, reuseSession)

	// Initialize long-term conversation history store
	historyMgr, err := contextManager.NewEventLogContext(sessionID)
	if err != nil {
		return nil, fmt.Errorf("system error: failed to initialize history manager: %v", err)
	}

	core := &AppCore{
		HistoryMgr:      historyMgr,
		IndexMgr:        indexMgr,
		EpisodeMgr:      episodeMgr,
		ReactorSTM:      reactorSTM,
		ResponderSTM:    responderSTM,
		Resp:            resp,
		ReactorAgent:    reactorAgent,
		MindStateVal:    "0.10:0.70:0.10:0.10:0.10", //neutral state
		DebugMode:       debugMode,
		PersonalityName: personalityName,
		MaxInputChars:   maxInputChars,
		MaxOutputChars:  maxOutputChars,
		OllamaCmd:       ollamaCmd,
	}

	if savedMindState != "" {
		core.SetMindState(savedMindState)
	}

	// Save the resolved session ID and mindstate back to the CSV ledger
	contextManager.UpdateSessionCSV(historyMgr.SessionID, core.GetMindState(), savedMentalEnergy)

	// Restore state if messages were loaded
	loadedMessages := historyMgr.GetMessages()
	if len(loadedMessages) > 0 {
		for _, msg := range loadedMessages {
			reactorSTM.Update(msg.Author, msg.Content)
			responderSTM.Update(msg.Author, msg.Content)
		}

		// If mindstate wasn't loaded from the file (e.g. --session used), fallback to last message
		if savedMindState == "" {
			if lastMsg := loadedMessages[len(loadedMessages)-1]; lastMsg.Metrics.MindScores != "" {
				core.SetMindState(lastMsg.Metrics.MindScores)
			}
		}

		fmt.Printf("\033[34m> [Session %s Restored (Mindstate: %s)]\033[0m\n", historyMgr.SessionID, core.GetMindState())
	}

	// Initialize Escalator (Scheduler and Rule Engine)
	sched := escalator.NewScheduler(
		core.GetMindState,
		core.GetUnconsolidatedChars,
	)
	core.Sched = sched // Link scheduler back to core

	sched.Engine.SetMentalEnergy(savedMentalEnergy) // Restore mental energy from CSV
	sched.Engine.CheckBiologicalEvents(core.GetMindState()) // Initialize biological state trackers
	sched.Engine.SetSleepMode(2)                            // Default to Hibernation
	go sched.Run(context.Background())

	return core, nil
}
