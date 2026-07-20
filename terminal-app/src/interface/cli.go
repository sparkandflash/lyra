package cli

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"terminal-app/src/utils"

	"github.com/chzyer/readline"

	"terminal-app/src/consolidator"
	"terminal-app/src/escalator"
	episode_memory "terminal-app/src/idle_methods/episode_memory"
	"terminal-app/src/interface/api"
	"terminal-app/src/reactor"
	"terminal-app/src/responder"
)

func updateSessionCSV(sessionID, mindState string, mentalEnergy float64) {
	csvPath := utils.ResolvePath(filepath.Join("Context", "conversationHistory", "sessions.csv"))
	var records [][]string
	
	file, err := os.Open(csvPath)
	if err == nil {
		reader := csv.NewReader(file)
		records, _ = reader.ReadAll()
		file.Close()
	}

	if len(records) == 0 {
		records = append(records, []string{"session_id", "mind_state", "mental_energy", "last_active"})
	}

	updated := false
	for i := 1; i < len(records); i++ {
		if len(records[i]) >= 4 && records[i][0] == sessionID {
			records[i][1] = mindState
			records[i][2] = fmt.Sprintf("%.2f", mentalEnergy)
			records[i][3] = time.Now().Format(time.RFC3339)
			updated = true
			break
		}
	}

	if !updated {
		records = append(records, []string{
			sessionID,
			mindState,
			fmt.Sprintf("%.2f", mentalEnergy),
			time.Now().Format(time.RFC3339),
		})
	}

	outFile, err := os.Create(csvPath)
	if err == nil {
		writer := csv.NewWriter(outFile)
		writer.WriteAll(records)
		writer.Flush()
		outFile.Close()
	}
}

// Run starts the interactive chat interface for Lyra.
func Run(newSession bool, reuseSession string, debugMode bool, noInterface bool) {
	personalityName := os.Getenv("SYSTEM_PERSONALITY_NAME")
	if personalityName == "" {
		personalityName = "lyra" // default fallback
	}

	// Start Ollama sidecar only if no remote URL is provided
	var ollamaCmd *exec.Cmd
	if os.Getenv("EMBEDDING_API_URL") == "" {
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
			fmt.Printf("system error: failed to start local embedding engine: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			if ollamaCmd != nil && ollamaCmd.Process != nil {
				ollamaCmd.Process.Kill()
			}
		}()
	}

	// Initialize the responder agent from environment configuration
	resp, err := responder.NewResponderFromEnv()
	if err != nil {
		fmt.Printf("system error: failed to initialize responder: %v\n", err)
		os.Exit(1)
	}

	// Initialize the reactor agent and its threshold
	reactorAgent := reactor.NewReactorAgent()
	reactorCharThreshold := responder.LoadReactorConfigFromEnv().ReactorCharThreshold


	// ── Reactor STM ──────────────────────────────────────────────────────────
	// SYSTEM_MAX_WORKING_MEMORY_CHARS controls the reactor's short-term memory window (default 2000).
	reactorMaxChars := 2000
	if limitStr := os.Getenv("SYSTEM_MAX_WORKING_MEMORY_CHARS"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			reactorMaxChars = limit
		}
	}
	reactorSTM := consolidator.NewSTMmanager(reactorMaxChars)

	// ── Responder STM ────────────────────────────────────────────────────────
	// SYSTEM_RESPONDER_STM_CHARS controls the responder's short-term memory window (default 2000).
	responderMaxChars := 2000
	if limitStr := os.Getenv("SYSTEM_RESPONDER_STM_CHARS"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			responderMaxChars = limit
		}
	}
	responderSTM := consolidator.NewSTMmanager(responderMaxChars)

	// ── Episode Memory Manager ────────────────────────────────────────────────
	// SYSTEM_EPISODE_MEMORY_CHARS controls the runtime episode pool's character budget (default 2000).
	episodeMgr := episode_memory.LoadEpisodeMemoryManagerFromEnv()

	// ── Character Limits ──────────────────────────────────────────────────────
	maxInputChars := 200
	if limitStr := os.Getenv("SYSTEM_MAX_INPUT_CHARS"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &maxInputChars); err != nil || maxInputChars <= 0 {
			maxInputChars = 200
		}
	}
	
	maxOutputChars := 200
	if limitStr := os.Getenv("SYSTEM_MAX_OUTPUT_CHARS"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &maxOutputChars); err != nil || maxOutputChars <= 0 {
			maxOutputChars = 200
		}
	}

	// ── Session Resolution ───────────────────────────────────────────────────
	historyDir := utils.ResolvePath(filepath.Join("Context", "conversationHistory"))
	os.MkdirAll(historyDir, 0755)
	
	csvPath := historyDir + "/sessions.csv"
	var sessionID string
	var savedMindState string
	var savedMentalEnergy float64 = 800.0

	if newSession {
		sessionID = "" // HistoryManager will generate a new one
	} else if reuseSession != "" {
		sessionID = reuseSession
		// Try to read existing state from CSV
		file, err := os.Open(csvPath)
		if err == nil {
			reader := csv.NewReader(file)
			records, _ := reader.ReadAll()
			for i := len(records) - 1; i >= 1; i-- {
				if len(records[i]) >= 3 && records[i][0] == sessionID {
					savedMindState = records[i][1]
					fmt.Sscanf(records[i][2], "%f", &savedMentalEnergy)
					break
				}
			}
			file.Close()
		}
	} else {
		// Attempt to read the most recent session from CSV
		file, err := os.Open(csvPath)
		if err == nil {
			reader := csv.NewReader(file)
			records, _ := reader.ReadAll()
			if len(records) > 1 {
				lastRow := records[len(records)-1]
				if len(lastRow) >= 3 {
					sessionID = lastRow[0]
					savedMindState = lastRow[1]
					fmt.Sscanf(lastRow[2], "%f", &savedMentalEnergy)
				}
			}
			file.Close()
		}
	}

	// Initialize long-term conversation history store
	historyMgr, err := consolidator.NewHistoryManager(sessionID)
	if err != nil {
		fmt.Printf("system error: failed to initialize history manager: %v\n", err)
		os.Exit(1)
	}

	// Instantiate AppCore early to serve as the single source of truth for state
	core := &AppCore{
		HistoryMgr:      historyMgr,
		EpisodeMgr:      episodeMgr,
		ReactorSTM:      reactorSTM,
		ResponderSTM:    responderSTM,
		Resp:            resp,
		ReactorAgent:    reactorAgent,
		MindStateVal:    "0.10:0.70:0.10:0.10:0.10",
		DebugMode:       debugMode,
		PersonalityName: personalityName,
		MaxInputChars:   maxInputChars,
		MaxOutputChars:  maxOutputChars,
	}

	if savedMindState != "" {
		core.SetMindState(savedMindState)
	}

	// Save the resolved session ID and mindstate back to the CSV ledger
	updateSessionCSV(historyMgr.SessionID, core.GetMindState(), savedMentalEnergy)

	// Restore state if messages were loaded
	loadedMessages := historyMgr.GetMessages()
	if len(loadedMessages) > 0 {
		for _, msg := range loadedMessages {
			reactorSTM.Update(msg.Author, msg.Content)
			responderSTM.Update(msg.Author, msg.Content)
		}
		
		// If mindstate wasn't loaded from the file (e.g. --session used), fallback to last message
		if savedMindState == "" {
			if lastMsg := loadedMessages[len(loadedMessages)-1]; lastMsg.MindState != "" {
				core.SetMindState(lastMsg.MindState)
			}
		}
		
		fmt.Printf("\033[34m> [Session %s Restored (Mindstate: %s)]\033[0m\n", historyMgr.SessionID, core.GetMindState())
	}

	// Initialize Escalator (Scheduler and Rule Engine)
	sched := escalator.NewScheduler(
		core.GetMindState,
		core.GetUnconsolidated,
	)
	core.Sched = sched // Link scheduler back to core
	
	sched.Engine.SetMentalEnergy(savedMentalEnergy) // Restore mental energy from CSV
	sched.Engine.CheckBiologicalEvents(core.GetMindState())   // Initialize biological state trackers
	sched.Engine.SetSleepMode(2) // Default to Hibernation
	go sched.Run(context.Background())

	// Initialize Readline
	var outWriter io.Writer = os.Stdout
	var rl *readline.Instance
	if !noInterface {
		var err error
		rl, err = readline.NewEx(&readline.Config{
			Prompt:      "> ",
			HistoryFile: historyDir + "/readline_history.txt",
		})
		if err != nil {
			fmt.Printf("system error: failed to initialize readline: %v\n", err)
			os.Exit(1)
		}
		defer rl.Close()
	} else {
		// No readline, everything just uses standard outWriter = os.Stdout
	}

	// Background input queue manager
	inputChan := make(chan string)
	if !noInterface {
		go func() {
			for {
				line, err := rl.Readline()
				if err != nil { // EOF or Ctrl+C
					if err == readline.ErrInterrupt {
						inputChan <- ">>sigint"
					} else {
						inputChan <- ">>eof"
					}
					break
				}
				inputChan <- strings.TrimSpace(line)
			}
		}()
	}

	apiInputChan := make(chan api.ChatInput)

	processChan := make(chan api.ChatInput)
	go func() {
		var queue []api.ChatInput
		for {
			var first api.ChatInput
			var sendChan chan<- api.ChatInput
			if len(queue) > 0 {
				first = queue[0]
				sendChan = processChan
			}
			
			select {
			case msg := <-inputChan:
				queue = append(queue, api.ChatInput{Message: msg})
			case msg := <-apiInputChan:
				queue = append(queue, msg)
			case sendChan <- first:
				queue = queue[1:]
			}
		}
	}()

	engineStartTime := time.Now()
	lastWakeTime := time.Now()

	// Start API server
	go api.StartServer(apiInputChan, historyMgr, sched, core.GetMindState)

	core.OutWriter = outWriter
	core.InputQueue = processChan
	core.RunLoop(engineStartTime, lastWakeTime, rl, reactorCharThreshold)
}
