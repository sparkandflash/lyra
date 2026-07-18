package cli

import (
	"context"
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"terminal-app/src/utils"

	"github.com/chzyer/readline"

	"terminal-app/src/consolidator"
	"terminal-app/src/escalator"
	"terminal-app/src/idle_methods/consolidation"
	episode_memory "terminal-app/src/idle_methods/episode_memory"
	"terminal-app/src/idle_methods/reflector"
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
		if records[i][0] == sessionID {
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
func Run(newSession bool, reuseSession string, debugMode bool) {
	personalityName := os.Getenv("SYSTEM_PERSONALITY_NAME")
	if personalityName == "" {
		personalityName = "lyra" // default fallback
	}

	// Check for local embedding engine sidecar
	binDir := utils.ResolvePath(".bin")
	ollamaPath := filepath.Join(binDir, "ollama")
	if _, err := os.Stat(ollamaPath); os.IsNotExist(err) {
		fmt.Println("system error: ollama bin and embedding model files are missing.")
		os.Exit(1)
	}

	// Start Ollama sidecar
	modelsDir := filepath.Join(binDir, "models")
	ollamaCmd := exec.Command(ollamaPath, "serve")
	ollamaCmd.Env = append(os.Environ(), 
		fmt.Sprintf("OLLAMA_MODELS=%s", modelsDir),
		"OLLAMA_HOST=127.0.0.1:11435",
	)
	
	if err := ollamaCmd.Start(); err != nil {
		fmt.Printf("system error: failed to start local embedding engine: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if ollamaCmd.Process != nil {
			ollamaCmd.Process.Kill()
		}
	}()

	// Initialize the responder agent from environment configuration
	resp, err := responder.NewResponderFromEnv()
	if err != nil {
		fmt.Printf("system error: failed to initialize responder: %v\n", err)
		os.Exit(1)
	}

	// Initialize the reactor agent and its threshold
	reactorAgent := reactor.NewReactorAgent()
	reactorCharThreshold := responder.LoadReactorConfigFromEnv().ReactorCharThreshold
	unreactedChars := 0

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

	// ── Session Resolution ───────────────────────────────────────────────────
	historyDir := utils.ResolvePath(filepath.Join("Context", "conversationHistory"))
	os.MkdirAll(historyDir, 0755)
	
	csvPath := historyDir + "/sessions.csv"
	var sessionID string
	var savedMindState string
	var savedMentalEnergy float64 = 100.0

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
				if records[i][0] == sessionID {
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
				sessionID = lastRow[0]
				savedMindState = lastRow[1]
				fmt.Sscanf(lastRow[2], "%f", &savedMentalEnergy)
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

	mindState := "0.90:0.30:0.50:0.70"
	if savedMindState != "" {
		mindState = savedMindState
	}

	// Save the resolved session ID and mindstate back to the CSV ledger
	updateSessionCSV(historyMgr.SessionID, mindState, savedMentalEnergy)

	// Restore state if messages were loaded
	loadedMessages := historyMgr.GetMessages()
	if len(loadedMessages) > 0 {
		for _, msg := range loadedMessages {
			reactorSTM.Update(msg.Author, msg.Content)
			responderSTM.Update(msg.Author, msg.Content)
		}
		
		// If mindState wasn't loaded from the file (e.g. --session used), fallback to last message
		if savedMindState == "" {
			if lastMsg := loadedMessages[len(loadedMessages)-1]; lastMsg.MindState != "" {
				mindState = lastMsg.MindState
			}
		}
		
		fmt.Printf("\033[34m> [Session %s Restored (Mindstate: %s)]\033[0m\n", historyMgr.SessionID, mindState)
	}
	// State for rule engine integration
	hasUnconsolidated := false
	inputLockedUntil := time.Time{}

	// Initialize Escalator (Scheduler and Rule Engine)
	sched := escalator.NewScheduler(
		func() string { return mindState },
		func() bool { return hasUnconsolidated },
	)
	sched.Engine.SetMentalEnergy(savedMentalEnergy) // Restore mental energy from CSV
	sched.Engine.SetSleepMode(2) // Default to Hibernation
	go sched.Run(context.Background())

	// Initialize Readline
	rl, err := readline.NewEx(&readline.Config{
		Prompt:      "> ",
		HistoryFile: historyDir + "/readline_history.txt",
	})
	if err != nil {
		fmt.Printf("system error: failed to initialize readline: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	// Background input queue manager
	inputChan := make(chan string)
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

	// Start API server
	go api.StartServer(apiInputChan, historyMgr, sched, func() string { return mindState })
	
	// Global OS Signal Listener for crashes/kills
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		// If we receive a kill signal, save state and exit directly
		sysMsg := "session ended abruptly"
		historyMgr.Save("system", sysMsg, mindState)
		updateSessionCSV(historyMgr.SessionID, mindState, sched.Engine.GetMentalEnergy())
		os.Exit(0)
	}()
	
	for {
		var activeProcessChan <-chan api.ChatInput
		var lockTimer <-chan time.Time
		if time.Now().After(inputLockedUntil) {
			activeProcessChan = processChan
		} else {
			lockTimer = time.After(time.Until(inputLockedUntil))
		}

		select {
		case <-lockTimer:
			// Wake up when input lock expires
		case evt := <-sched.EventChan:
			switch evt {
			case escalator.EventConsolidate:
				sysMsg := "[System: Memory consolidation triggered]"
				_ = historyMgr.Save("system", sysMsg, mindState)
				responderSTM.Update("system", sysMsg)
				reactorSTM.Update("system", sysMsg)

				newEpisodes, err := consolidation.Consolidate(historyMgr)
				if err == nil {
					for _, ep := range newEpisodes {
						episodeMgr.Push(episode_memory.EpisodeSummary{
							ID:            ep.ID,
							Summary:       ep.Summary,
							PeakMindState: ep.PeakMindState,
							Conclusion:    ep.Conclusion,
						})
					}
					hasUnconsolidated = false
				}
			case escalator.EventEnterTempSleep:
				sysMsg := "it has been more than 5 mins, starting idle time."
				if debugMode {
					fmt.Fprintf(rl.Stdout(), "[DEBUG] Entering Temp Sleep. Injecting system message.\n")
				}
				_ = historyMgr.Save("system", sysMsg, mindState)
				responderSTM.Update("system", sysMsg)
				reactorSTM.Update("system", sysMsg)
				hasUnconsolidated = true
			case escalator.EventEnterTrueSleep:
				sysMsg := "it has been more than 3 hours, starting hiberation."
				if debugMode {
					fmt.Fprintf(rl.Stdout(), "[DEBUG] Entering True Sleep (Hibernation). Injecting system message.\n")
				}
				_ = historyMgr.Save("system", sysMsg, mindState)
				responderSTM.Update("system", sysMsg)
				reactorSTM.Update("system", sysMsg)
				hasUnconsolidated = true
			case escalator.EventReflect:
				sysMsg := "[System: Reflecting on past memories]"
				_ = historyMgr.Save("system", sysMsg, mindState)
				responderSTM.Update("system", sysMsg)
				reactorSTM.Update("system", sysMsg)

				activeEps := episodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
				}
				matchedIDs, _ := reflector.Reflect(mindState, episodes)
				for _, id := range matchedIDs {
					_ = episodeMgr.LoadFromDisk(utils.ResolvePath(filepath.Join("Context", "episodes", id+".json")))
					if debugMode {
						fmt.Fprintf(rl.Stdout(), "[DEBUG] Reflect (Background): loaded episode %s\n", id)
					}
				}
			case escalator.EventIntrospect:
				sysMsg := "[System: Deep introspection initiated]"
				_ = historyMgr.Save("system", sysMsg, mindState)
				responderSTM.Update("system", sysMsg)
				reactorSTM.Update("system", sysMsg)

				activeEps := episodeMgr.GetActive()
				if len(activeEps) > 0 {
					_ = reflector.Introspect(activeEps[0].ID)
				}
			case escalator.EventProactiveMessage:
				ctx := context.Background()
				activeEps := episodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
				}

				// Inject the system cue into all active memory contexts
				sysMsg := "[System: Proactive message triggered]"
				_ = historyMgr.Save("system", sysMsg, mindState)
				reactorSTM.Update("system", sysMsg)
				responderSTM.Update("system", sysMsg)

				reply, usefulEpisodeID, err := resp.RespondProactive(ctx, mindState, responderSTM.GetNoFlags(), episodes)
				if err == nil {
					if usefulEpisodeID != "" {
						episodeMgr.MarkUseful(usefulEpisodeID)
					}
					
					for _, line := range strings.Split(reply, "\n") {
						if strings.TrimSpace(line) != "" {
							fmt.Fprintf(rl.Stdout(), "\033[34m> %s\033[0m\n", line)
						}
					}
					
					_ = historyMgr.Save(personalityName, reply, mindState)

					// Background: Reactor update
					// Save assistant's turn locally (Responder uses its own STM logic)
					reactorSTM.Update(personalityName, reply)
					responderSTM.Update("assistant", reply)
					hasUnconsolidated = true

					if respState, err := reactorAgent.React(ctx, reactorSTM.Get()); err == nil {
						mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.NegativeEmotion, respState.PositiveEmotion, respState.UserAttention)
					} else if debugMode {
						fmt.Fprintf(rl.Stdout(), "[DEBUG] Reactor Error (Proactive): %v\n", err)
					}
				}
			}

		case rawInput := <-activeProcessChan:
			if sched.Engine.GetCurrentSleepMode() == 2 {
				sched.Engine.SetSleepMode(0) // Wake up
				sysMsg := "[System: you just woke up from sleep]"
				_ = historyMgr.Save("system", sysMsg, mindState)
				reactorSTM.Update("system", sysMsg)
				responderSTM.Update("system", sysMsg)
				fmt.Fprintf(rl.Stdout(), "\033[90m%s\033[0m\n", sysMsg)
			}

			input := strings.TrimSpace(rawInput.Message)
			if input == "" {
				continue
			}

			if input == ">>debug" {
				fmt.Fprintf(rl.Stdout(), "system: mindstate: %s | HR: %.1f | energy: %.0f/100\n", mindState, sched.Engine.GetHeartrate(), sched.Engine.GetMentalEnergy())
				fmt.Fprintf(rl.Stdout(), "system: active episodes: %d | pinned: %q\n", len(episodeMgr.GetActive()), episodeMgr.GetPinnedID())
				continue
			} else if strings.HasPrefix(input, ">>mindstate ") {
				valStr := strings.TrimSpace(strings.TrimPrefix(input, ">>mindstate "))
				var ma, ne, pe, ua float64
				_, err := fmt.Sscanf(valStr, "%f:%f:%f:%f", &ma, &ne, &pe, &ua)
				if err != nil || ma < 0.0 || ma > 1.0 || ne < 0.0 || ne > 1.0 || pe < 0.0 || pe > 1.0 || ua < 0.0 || ua > 1.0 {
					fmt.Fprintln(rl.Stdout(), "system: error: mindstate must be four floats (0.0 to 1.0) separated by colons (e.g. 0.9:0.3:0.5:0.7).")
				} else {
					mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", ma, ne, pe, ua)
					fmt.Fprintf(rl.Stdout(), "system: mindstate updated to %s.\n", mindState)
				}
				continue
			} else if input == ">>consolidate" {
				newEpisodes, err := consolidation.Consolidate(historyMgr)
				if err != nil {
					fmt.Fprintf(rl.Stdout(), "system: error: consolidation failed: %v\n", err)
				} else {
					for _, ep := range newEpisodes {
						episodeMgr.Push(episode_memory.EpisodeSummary{
							ID:            ep.ID,
							Summary:       ep.Summary,
							PeakMindState: ep.PeakMindState,
							Conclusion:    ep.Conclusion,
						})
					}
					hasUnconsolidated = false
					fmt.Fprintf(rl.Stdout(), "system: consolidation completed successfully. %d episode(s) added.\n", len(newEpisodes))
				}
				continue
			} else if input == ">>reflect" {
				activeEps := episodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
				}
				matchedIDs, err := reflector.Reflect(mindState, episodes)
				if err != nil {
					fmt.Fprintf(rl.Stdout(), "system: error: reflection failed: %v\n", err)
				} else {
					loaded := 0
					for _, id := range matchedIDs {
						if err := episodeMgr.LoadFromDisk(utils.ResolvePath(filepath.Join("Context", "episodes", id+".json"))); err == nil {
							loaded++
						}
					}
					fmt.Fprintf(rl.Stdout(), "system: reflection completed. Found %d matching episodes, loaded %d into active memory.\n", len(matchedIDs), loaded)
				}
				continue
			} else if strings.HasPrefix(input, ">>introspect ") {
				episodeID := strings.TrimSpace(strings.TrimPrefix(input, ">>introspect "))
				if err := reflector.Introspect(episodeID); err != nil {
					fmt.Fprintf(rl.Stdout(), "system: error: introspection failed: %v\n", err)
				} else {
					fmt.Fprintf(rl.Stdout(), "system: introspection completed for %s. Reflection saved.\n", episodeID)
				}
				continue
			} else if input == ">>exit" {
				fmt.Fprintf(rl.Stdout(), "\r\033[K")
				sysMsg := "session has ended"
				_ = historyMgr.Save("system", sysMsg, mindState)
				updateSessionCSV(historyMgr.SessionID, mindState, sched.Engine.GetMentalEnergy())
				return
			} else if input == ">>sigint" || input == ">>eof" {
				fmt.Fprintf(rl.Stdout(), "\r\033[K")
				sysMsg := "session ended abruptly"
				_ = historyMgr.Save("system", sysMsg, mindState)
				updateSessionCSV(historyMgr.SessionID, mindState, sched.Engine.GetMentalEnergy())
				fmt.Fprintf(rl.Stdout(), "\033[31m> session terminated abruptly.\033[0m\n")
				return
			}

			sched.Engine.OnUserMessage(mindState)
			hasUnconsolidated = true

			ctx := context.Background()
			
			// 3-second minimum delay
			startTime := time.Now()
			done := make(chan bool)
			go func() {
				// With async printing, we avoid scrambling the terminal with raw \r returns.
				fmt.Fprintf(rl.Stdout(), "\033[34m> [thinking...]\033[0m\n")
				<-done
			}()

			// Save user message to long-term history
			_ = historyMgr.Save("user", input, mindState)

			// Update both STMs
			reactorSTM.Update("user", input)
			responderSTM.Update("user", input)

			var currentMA float64
			fmt.Sscanf(mindState, "%f:", &currentMA)

			// Skip logic: if MA < 0.20, 1/3 chance to skip processing
			if currentMA < 0.20 && rand.Float64() < 0.3333 {
				time.Sleep(time.Until(startTime.Add(3 * time.Second)))
				done <- true
				
				if debugMode {
					fmt.Fprintf(rl.Stdout(), "[DEBUG] Model attention is < 0.20. Randomly skipping this turn (1/3 chance).\n")
				}
				
				
				reply := "no response"
				
				if rawInput.ResponseChan != nil {
					rawInput.ResponseChan <- reply
				}

				fmt.Fprintf(rl.Stdout(), "\033[34m> %s\033[0m\n", reply)
				_ = historyMgr.Save(personalityName, reply, mindState)
				responderSTM.Update("assistant", reply)
				reactorSTM.Update("assistant", reply)
				continue
			}

			// Throttle: Only invoke reactor if character threshold is met
			unreactedChars += len(input)
			if unreactedChars >= reactorCharThreshold {
				if respState, err := reactorAgent.React(ctx, reactorSTM.Get()); err == nil {
					mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.NegativeEmotion, respState.PositiveEmotion, respState.UserAttention)
					unreactedChars = 0
					if debugMode {
						fmt.Fprintf(rl.Stdout(), "[DEBUG] Reactor (Pre-Response): Mindstate updated to %s\n", mindState)
					}
				} else if debugMode {
					fmt.Fprintf(rl.Stdout(), "[DEBUG] Reactor Error (Pre-Response): %v\n", err)
				}
			}

			// Build the episode summaries from the active episode pool
			activeEps := episodeMgr.GetActive()
			episodes := make([]responder.EpisodeSummary, len(activeEps))
			for i, ep := range activeEps {
				episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
			}

			// Respond using responder's clean STM (no stored flags) + active episodes
			// Pass mental energy as a length hint appended to the mindstate string.
			energyHint := fmt.Sprintf("%s|energy:%.0f", mindState, sched.Engine.GetMentalEnergy())
			reply, usefulEpisodeID, err := resp.Respond(ctx, input, energyHint, responderSTM.GetNoFlags(), episodes)
			if err != nil {
				done <- true
				fmt.Fprintf(rl.Stdout(), "\033[31merror: failed to generate response: %v\033[0m\n", err)
			} else {
				if debugMode {
					fmt.Fprintf(rl.Stdout(), "[DEBUG] Responder: Output received (Useful Episode ID: %q)\n", usefulEpisodeID)
				}
				
				// If the model identified a useful episode, pin it to prevent eviction
				if usefulEpisodeID != "" {
					episodeMgr.MarkUseful(usefulEpisodeID)
				}

				// Throttle: Only invoke reactor if character threshold is met
				unreactedChars += len(reply)
				if unreactedChars >= reactorCharThreshold {
					if respState, err := reactorAgent.React(ctx, reactorSTM.Get()); err == nil {
						mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.NegativeEmotion, respState.PositiveEmotion, respState.UserAttention)
						unreactedChars = 0
						if debugMode {
							fmt.Fprintf(rl.Stdout(), "[DEBUG] Reactor (Post-Response): Mindstate updated to %s\n", mindState)
						}
					} else if debugMode {
						fmt.Fprintf(rl.Stdout(), "[DEBUG] Reactor Error (Post-Response): %v\n", err)
					}
				}

				// Ensure at least 3 seconds have passed
				time.Sleep(time.Until(startTime.Add(3 * time.Second)))
				done <- true
				
				// Save assistant response to long-term history and responder STM
				_ = historyMgr.Save(personalityName, reply, mindState)
				responderSTM.Update("assistant", reply)
				reactorSTM.Update("assistant", reply)

				if rawInput.ResponseChan != nil {
					rawInput.ResponseChan <- reply
				}

				for _, line := range strings.Split(reply, "\n") {
					if strings.TrimSpace(line) != "" {
						fmt.Fprintf(rl.Stdout(), "\033[34m> %s\033[0m\n", line)
					}
				}
				sched.Engine.OnResponse()
			}
		}
	}
}
