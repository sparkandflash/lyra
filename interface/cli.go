package cli

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"

	"lyra/consolidator"
	"lyra/escalator"
	"lyra/idle_methods/consolidation"
	episode_memory "lyra/idle_methods/episode_memory"
	"lyra/idle_methods/reflector"
	"lyra/reactor"
	"lyra/responder"
)

// Run starts the interactive chat interface for Lyra.
func Run(newSession bool, reuseSession string, debugMode bool) {
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
	// LYRA_MAX_WORKING_MEMORY_CHARS controls the reactor's short-term memory window (default 2000).
	reactorMaxChars := 2000
	if limitStr := os.Getenv("LYRA_MAX_WORKING_MEMORY_CHARS"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			reactorMaxChars = limit
		}
	}
	reactorSTM := consolidator.NewSTMmanager(reactorMaxChars)

	// ── Responder STM ────────────────────────────────────────────────────────
	// LYRA_RESPONDER_STM_CHARS controls the responder's short-term memory window (default 2000).
	responderMaxChars := 2000
	if limitStr := os.Getenv("LYRA_RESPONDER_STM_CHARS"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			responderMaxChars = limit
		}
	}
	responderSTM := consolidator.NewSTMmanager(responderMaxChars)

	// ── Episode Memory Manager ────────────────────────────────────────────────
	// LYRA_EPISODE_MEMORY_CHARS controls the runtime episode pool's character budget (default 2000).
	episodeMgr := episode_memory.LoadEpisodeMemoryManagerFromEnv()

	// ── Session Resolution ───────────────────────────────────────────────────
	historyDir := "Context/conversationHistory"
	os.MkdirAll(historyDir, 0755)
	lastSessionPath := historyDir + "/last_session.txt"
	var sessionID string
	var savedMindState string

	if newSession {
		sessionID = "" // HistoryManager will generate a new one
	} else if reuseSession != "" {
		sessionID = reuseSession
	} else {
		// Attempt to read the last session
		if data, err := os.ReadFile(lastSessionPath); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) > 0 {
				sessionID = strings.TrimSpace(lines[0])
			}
			if len(lines) > 1 {
				savedMindState = strings.TrimSpace(lines[1])
			}
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

	// Save the resolved session ID and mindstate back to last_session.txt
	os.WriteFile(lastSessionPath, []byte(fmt.Sprintf("%s\n%s", historyMgr.SessionID, mindState)), 0644)

	// Restore state if messages were loaded
	loadedMessages := historyMgr.GetMessages()
	if len(loadedMessages) > 0 {
		for _, msg := range loadedMessages {
			reactorSTM.Update(msg.Role, msg.Content)
			responderSTM.Update(msg.Role, msg.Content)
		}
		
		// If mindState wasn't loaded from the file (e.g. --session used), fallback to last message
		if savedMindState == "" {
			if lastMsg := loadedMessages[len(loadedMessages)-1]; lastMsg.MindState != "" {
				mindState = lastMsg.MindState
			}
		}
		
		fmt.Printf("\033[34m> [Session %s Restored (Mindstate: %s)]\033[0m\n", historyMgr.SessionID, mindState)
	} else {
		fmt.Println("\033[34m> hello, nice to meet you.\033[0m")
	}

	// State for rule engine integration
	hasUnconsolidated := false
	inputLockedUntil := time.Time{}

	// Initialize Escalator (Scheduler and Rule Engine)
	sched := escalator.NewScheduler(
		func() string { return mindState },
		func() bool { return hasUnconsolidated },
	)
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

	// Background input reader
	inputChan := make(chan string)
	go func() {
		for {
			line, err := rl.Readline()
			if err != nil { // EOF or Ctrl+C
				break
			}
			inputChan <- strings.TrimSpace(line)
		}
	}()
	
	for {
		select {
		case evt := <-sched.EventChan:
			switch evt {
			case escalator.EventConsolidate:
				newEpisodes, err := consolidation.Consolidate(historyMgr)
				if err == nil {
					for _, ep := range newEpisodes {
						episodeMgr.Push(episode_memory.EpisodeSummary{
							ID:            ep.ID,
							Summary:       ep.Summary,
							Keywords:      ep.Keywords,
							PeakMindState: ep.PeakMindState,
							Conclusion:    ep.Conclusion,
						})
					}
					hasUnconsolidated = false
				}
			case escalator.EventReflect:
				activeEps := episodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, Keywords: ep.Keywords, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
				}
				matchedIDs, _ := reflector.Reflect(mindState, episodes)
				for _, id := range matchedIDs {
					_ = episodeMgr.LoadFromDisk("Context/episodes/" + id + ".json")
					if debugMode {
						fmt.Fprintf(rl.Stdout(), "[DEBUG] Reflect (Background): loaded episode %s\n", id)
					}
				}
			case escalator.EventIntrospect:
				activeEps := episodeMgr.GetActive()
				if len(activeEps) > 0 {
					_ = reflector.Introspect(activeEps[0].ID)
				}
			case escalator.EventProactiveMessage:
				ctx := context.Background()
				activeEps := episodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, Keywords: ep.Keywords, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
				}

				reply, usefulEpisodeID, err := resp.RespondProactive(ctx, mindState, responderSTM.GetNoFlags(), episodes)
				if err == nil {
					if usefulEpisodeID != "" {
						episodeMgr.MarkUseful(usefulEpisodeID)
					}
					
					for _, line := range strings.Split(reply, "\n") {
						if line != "" {
							fmt.Fprintf(rl.Stdout(), "\033[34m> %s\033[0m\n", line)
						}
					}
					
					_ = historyMgr.Save("assistant", reply, mindState)
					responderSTM.Update("assistant", reply)
					reactorSTM.Update("assistant", reply)
					hasUnconsolidated = true

					if respState, err := reactorAgent.React(ctx, reactorSTM.Get()); err == nil {
						mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.NegativeEmotion, respState.PositiveEmotion, respState.UserAttention)
					} else if debugMode {
						fmt.Fprintf(rl.Stdout(), "[DEBUG] Reactor Error (Proactive): %v\n", err)
					}
				}
			}

		case rawInput := <-inputChan:
			if time.Now().Before(inputLockedUntil) {
				// Discard input during lock
				continue
			}

			input := strings.TrimSpace(rawInput)
			if input == "" {
				continue
			}

			if input == ">>debug" {
				fmt.Fprintf(rl.Stdout(), "debug: mindstate: %s | HR: %.1f | energy: %.0f/100\n", mindState, sched.Engine.Heartrate, sched.Engine.MentalEnergy)
				fmt.Fprintf(rl.Stdout(), "debug: active episodes: %d | pinned: %q\n", len(episodeMgr.GetActive()), episodeMgr.GetPinnedID())
				continue
			} else if strings.HasPrefix(input, ">>mindstate ") {
				valStr := strings.TrimSpace(strings.TrimPrefix(input, ">>mindstate "))
				var ma, ne, pe, ua float64
				_, err := fmt.Sscanf(valStr, "%f:%f:%f:%f", &ma, &ne, &pe, &ua)
				if err != nil || ma < 0.0 || ma > 1.0 || ne < 0.0 || ne > 1.0 || pe < 0.0 || pe > 1.0 || ua < 0.0 || ua > 1.0 {
					fmt.Fprintln(rl.Stdout(), "debug: error: mindstate must be four floats (0.0 to 1.0) separated by colons (e.g. 0.9:0.3:0.5:0.7).")
				} else {
					mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", ma, ne, pe, ua)
					fmt.Fprintf(rl.Stdout(), "debug: mindstate updated to %s.\n", mindState)
				}
				continue
			} else if input == ">>consolidate" {
				newEpisodes, err := consolidation.Consolidate(historyMgr)
				if err != nil {
					fmt.Fprintf(rl.Stdout(), "debug: error: consolidation failed: %v\n", err)
				} else {
					for _, ep := range newEpisodes {
						episodeMgr.Push(episode_memory.EpisodeSummary{
							ID:            ep.ID,
							Summary:       ep.Summary,
							Keywords:      ep.Keywords,
							PeakMindState: ep.PeakMindState,
							Conclusion:    ep.Conclusion,
						})
					}
					hasUnconsolidated = false
					fmt.Fprintf(rl.Stdout(), "debug: consolidation completed successfully. %d episode(s) added.\n", len(newEpisodes))
				}
				continue
			} else if input == ">>reflect" {
				activeEps := episodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, Keywords: ep.Keywords, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
				}
				matchedIDs, err := reflector.Reflect(mindState, episodes)
				if err != nil {
					fmt.Fprintf(rl.Stdout(), "debug: error: reflection failed: %v\n", err)
				} else {
					loaded := 0
					for _, id := range matchedIDs {
						if err := episodeMgr.LoadFromDisk("Context/episodes/" + id + ".json"); err == nil {
							loaded++
						}
					}
					fmt.Fprintf(rl.Stdout(), "debug: reflection completed. Found %d matching episodes, loaded %d into active memory.\n", len(matchedIDs), loaded)
				}
				continue
			} else if strings.HasPrefix(input, ">>introspect ") {
				episodeID := strings.TrimSpace(strings.TrimPrefix(input, ">>introspect "))
				if err := reflector.Introspect(episodeID); err != nil {
					fmt.Fprintf(rl.Stdout(), "debug: error: introspection failed: %v\n", err)
				} else {
					fmt.Fprintf(rl.Stdout(), "debug: introspection completed for %s. Reflection saved.\n", episodeID)
				}
				continue
			} else if input == "exit" || input == "quit" {
				os.WriteFile(lastSessionPath, []byte(fmt.Sprintf("%s\n%s", historyMgr.SessionID, mindState)), 0644)
				fmt.Println("\033[34m> goodbye!\033[0m")
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
				fmt.Fprintf(rl.Stdout(), "\033[34m> %s\033[0m\n", reply)
				_ = historyMgr.Save("assistant", reply, mindState)
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
				episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, Keywords: ep.Keywords, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
			}

			// Respond using responder's clean STM (no stored flags) + active episodes
			// Pass mental energy as a length hint appended to the mindstate string.
			energyHint := fmt.Sprintf("%s|energy:%.0f", mindState, sched.Engine.MentalEnergy)
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
				_ = historyMgr.Save("assistant", reply, mindState)
				responderSTM.Update("assistant", reply)
				reactorSTM.Update("assistant", reply)

				for _, line := range strings.Split(reply, "\n") {
					if line != "" {
						fmt.Fprintf(rl.Stdout(), "\033[34m> %s\033[0m\n", line)
					}
				}
				sched.Engine.OnResponse()
			}
		}
	}
}
