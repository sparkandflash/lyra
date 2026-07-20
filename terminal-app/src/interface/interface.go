package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"terminal-app/src/consolidator"
	"terminal-app/src/escalator"
	"terminal-app/src/idle_methods/consolidation"
	"terminal-app/src/idle_methods/episode_memory"
	"terminal-app/src/idle_methods/reflector"
	"terminal-app/src/interface/api"
	"terminal-app/src/reactor"
	"terminal-app/src/responder"
)

type AppCore struct {
	HistoryMgr      *consolidator.HistoryManager
	EpisodeMgr      *episode_memory.EpisodeMemoryManager
	ReactorSTM      *consolidator.STMmanager
	ResponderSTM    *consolidator.STMmanager
	Sched           *escalator.Scheduler
	Resp            responder.Responder
	ReactorAgent    *reactor.ReactorAgent

	MindStateVal    string
	StateMu         sync.RWMutex
	
	HasUnconsolidatedVal bool
	
	DebugMode       bool
	PersonalityName string
	OutWriter       io.Writer
	
	InputQueue      chan api.ChatInput
	
	UnreactedChars  int
}

func (c *AppCore) GetMindState() string {
	c.StateMu.RLock()
	defer c.StateMu.RUnlock()
	return c.MindStateVal
}

func (c *AppCore) SetMindState(ms string) {
	c.StateMu.Lock()
	defer c.StateMu.Unlock()
	c.MindStateVal = ms
}

func (c *AppCore) GetUnconsolidated() bool {
	c.StateMu.RLock()
	defer c.StateMu.RUnlock()
	return c.HasUnconsolidatedVal
}

func (c *AppCore) SetUnconsolidated(val bool) {
	c.StateMu.Lock()
	defer c.StateMu.Unlock()
	c.HasUnconsolidatedVal = val
}

func (c *AppCore) InjectSystemMessage(sysMsg string) {
	_ = c.HistoryMgr.Save("system", sysMsg, c.GetMindState())
	c.ReactorSTM.Update("system", sysMsg)
	c.ResponderSTM.Update("system", sysMsg)
}

type readliner interface {
	Close() error
}

func (c *AppCore) RunLoop(engineStartTime time.Time, lastWakeTime time.Time, rl readliner, reactorCharThreshold int) {
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		// If we receive a kill signal, save state and exit directly
		sysMsg := "session ended abruptly"
		c.HistoryMgr.Save("system", sysMsg, c.GetMindState())
		updateSessionCSV(c.HistoryMgr.SessionID, c.GetMindState(), c.Sched.Engine.GetMentalEnergy())
		os.Exit(0)
	}()
	
	for {
		select {
		case evt := <-c.Sched.EventChan:
			switch evt {
			case escalator.EventConsolidate:
				sysMsg := "[System: Memory consolidation triggered]"
				if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", sysMsg)
				}
				c.InjectSystemMessage(sysMsg)

				var activeEps []consolidation.EpisodeSummary
				for _, e := range c.EpisodeMgr.GetActive() {
					activeEps = append(activeEps, consolidation.EpisodeSummary{
						ID:            e.ID,
						Facts:         e.Facts,
						PeakMindState: e.PeakMindState,
					})
				}
				newEpisodes, err := consolidation.Consolidate(c.HistoryMgr, activeEps)
				if err == nil {
					for _, ep := range newEpisodes {
						c.EpisodeMgr.Push(episode_memory.EpisodeSummary{
							ID:            ep.ID,
							Facts:         ep.Facts,
							PeakMindState: ep.PeakMindState,
						})
					}
					c.SetUnconsolidated(false)
				}
			case escalator.EventEnterTempSleep:
				sysMsg := "[System: User has disconnected from the interface.]"
				if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", sysMsg)
				}
				c.InjectSystemMessage(sysMsg)
				c.SetUnconsolidated(true)
			case escalator.EventEnterTrueSleep:
				delay := os.Getenv("SYSTEM_TRUE_SLEEP_DELAY_MINS")
				if delay == "" { delay = "180" }
				sysMsg := fmt.Sprintf("[System: it has been %s mins since user last responded, starting hibernation.]", delay)
				if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", sysMsg)
				}
				c.InjectSystemMessage(sysMsg)
				c.SetUnconsolidated(true)
			case escalator.EventReflect:
				sysMsg := "[System: Reflecting on past memories]"
				if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", sysMsg)
				}
				c.InjectSystemMessage(sysMsg)

				activeEps := c.EpisodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Facts: ep.Facts, PeakMindState: ep.PeakMindState}
				}
				matchedFacts, _ := reflector.Reflect(c.GetMindState(), episodes)
				for _, fact := range matchedFacts {
					c.EpisodeMgr.Push(fact)
					if c.DebugMode {
						fmt.Fprintf(c.OutWriter, "[DEBUG] Reflect (Background): loaded fact %s\n", fact.ID)
					}
				}
			case escalator.EventIntrospect:
				sysMsg := "[System: Deep introspection initiated]"
				if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", sysMsg)
				}
				c.InjectSystemMessage(sysMsg)

				_ = reflector.Introspect(c.HistoryMgr, c.EpisodeMgr)
			case escalator.EventProactiveMessage:
				ctx := context.Background()
				activeEps := c.EpisodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Facts: ep.Facts, PeakMindState: ep.PeakMindState}
				}

				// Inject the system cue into all active memory contexts
				sysMsg := "[System: Proactive message triggered]"
				if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", sysMsg)
				}
				c.InjectSystemMessage(sysMsg)

				reply, usefulEpisodeID, err := c.Resp.RespondProactive(ctx, c.GetMindState(), c.ResponderSTM.GetNoFlags(), episodes)
				if err == nil {
					if usefulEpisodeID != "" {
						c.EpisodeMgr.MarkUseful(usefulEpisodeID)
					}
					
					for _, line := range strings.Split(reply, "\n") {
						if strings.TrimSpace(line) != "" {
							fmt.Fprintf(c.OutWriter, "\033[34m> %s\033[0m\n", line)
						}
					}
					
					_ = c.HistoryMgr.Save(c.PersonalityName, reply, c.GetMindState())

					// Background: Reactor update
					// Save assistant's turn locally (Responder uses its own STM logic)
					c.ReactorSTM.Update(c.PersonalityName, reply)
					c.ResponderSTM.Update("assistant", reply)
					c.SetUnconsolidated(true)

					if respState, err := c.ReactorAgent.React(ctx, c.ReactorSTM.Get()); err == nil {
						newMindState := fmt.Sprintf("%.2f:%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.UserAttention, respState.Serotonin, respState.Oxytocin, respState.Cortisol)
						if newMindState != "0.00:0.00:0.00:0.00:0.00" {
							c.SetMindState(newMindState)
							if bioEvents := c.Sched.Engine.CheckBiologicalEvents(c.GetMindState()); bioEvents != "" {
								if c.DebugMode {
									fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", bioEvents)
								}
								c.InjectSystemMessage(bioEvents)
							}
						} else if c.DebugMode {
							fmt.Fprintf(c.OutWriter, "[DEBUG] Reactor Error: parsed 0:0:0:0. Keeping previous mindstate %s\n", c.GetMindState())
						}
					} else if c.DebugMode {
						fmt.Fprintf(c.OutWriter, "[DEBUG] Reactor Error (Proactive): %v\n", err)
					}
				}
			}

		case rawInput := <-c.InputQueue:
			input := strings.TrimSpace(rawInput.Message)
			
			// Handle exits immediately before triggering wake logic
			if input == ">>exit" {
				if rl != nil { rl.Close() }
				fmt.Printf("\r\033[2K\r") // Clear entire line and return to start
				sysMsg := "session has ended"
				_ = c.HistoryMgr.Save("system", sysMsg, c.GetMindState())
				updateSessionCSV(c.HistoryMgr.SessionID, c.GetMindState(), c.Sched.Engine.GetMentalEnergy())
				os.Exit(0)
			} else if input == ">>sigint" || input == ">>eof" {
				if rl != nil { rl.Close() }
				fmt.Printf("\r\033[2K\r")
				sysMsg := "session ended abruptly"
				_ = c.HistoryMgr.Save("system", sysMsg, c.GetMindState())
				updateSessionCSV(c.HistoryMgr.SessionID, c.GetMindState(), c.Sched.Engine.GetMentalEnergy())
				fmt.Println("\033[31m> session terminated abruptly.\033[0m")
				os.Exit(0)
			}

			if c.Sched.Engine.GetCurrentSleepMode() == 2 {
				c.Sched.Engine.SetSleepMode(0) // Wake up
				lastWakeTime = time.Now()
				sysMsg := fmt.Sprintf("[System: you just woke up from sleep. The current time is %s]", time.Now().Format("Monday, Jan 2, 3:04 PM"))
				c.InjectSystemMessage(sysMsg)
				if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", sysMsg)
				}
			}

			if input == "" {
				continue
			}

			if input == ">>debug" {
				uptime := time.Since(lastWakeTime).Round(time.Second)
				totalUptime := time.Since(engineStartTime).Round(time.Second)
				fmt.Fprintf(c.OutWriter, "system: mindstate: %s | drain: %.2f/s | energy: %.0f/1000\n", c.GetMindState(), c.Sched.Engine.GetEnergyDrainRate(), c.Sched.Engine.GetMentalEnergy())
				fmt.Fprintf(c.OutWriter, "system: active episodes: %d | pinned: %q\n", len(c.EpisodeMgr.GetActive()), c.EpisodeMgr.GetPinnedID())
				fmt.Fprintf(c.OutWriter, "system: uptime: %v | totalUptime: %v\n", uptime, totalUptime)
				continue
			} else if strings.HasPrefix(input, ">>mindstate ") {
				valStr := strings.TrimSpace(strings.TrimPrefix(input, ">>mindstate "))
				var ma, ua, se, ox, co float64
				_, err := fmt.Sscanf(valStr, "%f:%f:%f:%f:%f", &ma, &ua, &se, &ox, &co)
				if err != nil || ma < -1.0 || ma > 1.0 || ua < -1.0 || ua > 1.0 || se < -1.0 || se > 1.0 || ox < -1.0 || ox > 1.0 || co < -1.0 || co > 1.0 {
					fmt.Fprintln(c.OutWriter, "system: error: mindstate must be five floats separated by colons (e.g. 0.9:0.7:0.0:0.0:0.0).")
				} else {
					c.SetMindState(fmt.Sprintf("%.2f:%.2f:%.2f:%.2f:%.2f", ma, ua, se, ox, co))
					fmt.Fprintf(c.OutWriter, "system: mindstate updated to %s.\n", c.GetMindState())
				}
				continue
			} else if input == ">>consolidate" {
				var activeEps []consolidation.EpisodeSummary
				for _, e := range c.EpisodeMgr.GetActive() {
					activeEps = append(activeEps, consolidation.EpisodeSummary{
						ID:            e.ID,
						Facts:         e.Facts,
						PeakMindState: e.PeakMindState,
					})
				}
				
				newEpisodes, err := consolidation.Consolidate(c.HistoryMgr, activeEps)
				if err != nil {
					fmt.Fprintf(c.OutWriter, "system: error: consolidation failed: %v\n", err)
				} else {
					for _, ep := range newEpisodes {
						c.EpisodeMgr.Push(episode_memory.EpisodeSummary{
							ID:            ep.ID,
							Facts:         ep.Facts,
							PeakMindState: ep.PeakMindState,
						})
					}
					c.SetUnconsolidated(false)
					fmt.Fprintf(c.OutWriter, "system: consolidation completed successfully. %d episode(s) added.\n", len(newEpisodes))
				}
				continue
			} else if input == ">>reflect" {
				activeEps := c.EpisodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Facts: ep.Facts, PeakMindState: ep.PeakMindState}
				}
				matchedFacts, err := reflector.Reflect(c.GetMindState(), episodes)
				if err != nil {
					if c.DebugMode {
						fmt.Fprintf(c.OutWriter, "[DEBUG] Reflect explicitly failed: %v\n", err)
					}
				} else {
					for _, fact := range matchedFacts {
						c.EpisodeMgr.Push(fact)
						if c.DebugMode {
							fmt.Fprintf(c.OutWriter, "[DEBUG] Reflect explicitly loaded fact %s\n", fact.ID)
						}
					}
				}
				continue
			} else if strings.HasPrefix(input, ">>introspect") {
				if err := reflector.Introspect(c.HistoryMgr, c.EpisodeMgr); err != nil {
					fmt.Fprintf(c.OutWriter, "system: error: introspection failed: %v\n", err)
				} else {
					fmt.Fprintf(c.OutWriter, "system: introspection completed. Behavioral fact saved.\n")
				}
				continue
			}

			c.Sched.Engine.OnUserMessage(c.GetMindState())
			c.SetUnconsolidated(true)

			ctx := context.Background()
			
			// 3-second minimum delay
			startTime := time.Now()
			done := make(chan bool, 1)
			go func() {
				// With async printing, we avoid scrambling the terminal with raw \r returns.
				fmt.Fprintf(c.OutWriter, "\033[34m> [thinking...]\033[0m\n")
				<-done
			}()

			// Save user message to long-term history
			_ = c.HistoryMgr.Save("user", input, c.GetMindState())

			// Update both STMs
			c.ReactorSTM.Update("user", input)
			c.ResponderSTM.Update("user", input)

			var currentMA float64
			fmt.Sscanf(c.GetMindState(), "%f:", &currentMA)

			// Skip logic: if MA < 0.0 and Energy < 400, explicitly ignore the user
			if currentMA < 0.0 && c.Sched.Engine.GetMentalEnergy() < 400.0 {
				time.Sleep(time.Until(startTime.Add(3 * time.Second)))
				done <- true
				
				if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "[DEBUG] Model attention < 0.0 and Energy < 400. Skipping this turn.\n")
				}
				
				sysMsg := "[System: You felt too exhausted and uninterested to reply. You ignored the user.]"
				if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", sysMsg)
				}
				c.InjectSystemMessage(sysMsg)
				
				reply := "no response"
				
				if rawInput.ResponseChan != nil {
					rawInput.ResponseChan <- reply
				}

				fmt.Fprintf(c.OutWriter, "\033[34m> %s\033[0m\n", reply)
				_ = c.HistoryMgr.Save(c.PersonalityName, reply, c.GetMindState())
				c.ResponderSTM.Update("assistant", reply)
				c.ReactorSTM.Update("assistant", reply)
				continue
			}

			// Throttle: Only invoke reactor if character threshold is met
			c.UnreactedChars += len(input)
			if c.UnreactedChars >= reactorCharThreshold {
				if respState, err := c.ReactorAgent.React(ctx, c.ReactorSTM.Get()); err == nil {
					newMindState := fmt.Sprintf("%.2f:%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.UserAttention, respState.Serotonin, respState.Oxytocin, respState.Cortisol)
					if newMindState != "0.00:0.00:0.00:0.00:0.00" {
						c.SetMindState(newMindState)
						if bioEvents := c.Sched.Engine.CheckBiologicalEvents(c.GetMindState()); bioEvents != "" {
							if c.DebugMode {
								fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", bioEvents)
							}
							c.InjectSystemMessage(bioEvents)
						}
						c.UnreactedChars = 0
						if c.DebugMode {
							fmt.Fprintf(c.OutWriter, "[DEBUG] Reactor (Pre-Response): Mindstate updated to %s\n", c.GetMindState())
						}
					} else if c.DebugMode {
						fmt.Fprintf(c.OutWriter, "[DEBUG] Reactor Error (Pre-Response): parsed 0:0:0:0. Keeping previous mindstate %s\n", c.GetMindState())
					}
				} else if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "[DEBUG] Reactor Error (Pre-Response): %v\n", err)
				}
			}

			// Build the episode summaries from the active episode pool
			activeEps := c.EpisodeMgr.GetActive()
			episodes := make([]responder.EpisodeSummary, len(activeEps))
			for i, ep := range activeEps {
				episodes[i] = responder.EpisodeSummary{ID: ep.ID, Facts: ep.Facts, PeakMindState: ep.PeakMindState}
			}

			// Respond using responder's clean STM (no stored flags) + active episodes
			// Pass mental energy as a length hint appended to the mindstate string.
			energyHint := fmt.Sprintf("%s|energy:%.0f|drain_rate:%.1f/s", c.GetMindState(), c.Sched.Engine.GetMentalEnergy(), c.Sched.Engine.GetEnergyDrainRate())
			reply, usefulEpisodeID, err := c.Resp.Respond(ctx, input, energyHint, c.ResponderSTM.GetNoFlags(), episodes)
			if err != nil {
				time.Sleep(time.Until(startTime.Add(3 * time.Second)))
				done <- true
				fmt.Fprintf(c.OutWriter, "\033[31merror: failed to generate response: %v\033[0m\n", err)
			} else {
				if c.DebugMode {
					fmt.Fprintf(c.OutWriter, "[DEBUG] Responder: Output received (Useful Episode ID: %q)\n", usefulEpisodeID)
				}
				
				// If the model identified a useful episode, pin it to prevent eviction
				if usefulEpisodeID != "" {
					c.EpisodeMgr.MarkUseful(usefulEpisodeID)
				}

				// Throttle: Only invoke reactor if character threshold is met
				c.UnreactedChars += len(reply)
				if c.UnreactedChars >= reactorCharThreshold {
					if respState, err := c.ReactorAgent.React(ctx, c.ReactorSTM.Get()); err == nil {
						newMindState := fmt.Sprintf("%.2f:%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.UserAttention, respState.Serotonin, respState.Oxytocin, respState.Cortisol)
						if newMindState != "0.00:0.00:0.00:0.00:0.00" {
							c.SetMindState(newMindState)
							if bioEvents := c.Sched.Engine.CheckBiologicalEvents(c.GetMindState()); bioEvents != "" {
								if c.DebugMode {
									fmt.Fprintf(c.OutWriter, "\033[90m%s\033[0m\n", bioEvents)
								}
								c.InjectSystemMessage(bioEvents)
							}
							c.UnreactedChars = 0
							if c.DebugMode {
								fmt.Fprintf(c.OutWriter, "[DEBUG] Reactor (Post-Response): Mindstate updated to %s\n", c.GetMindState())
							}
						} else if c.DebugMode {
							fmt.Fprintf(c.OutWriter, "[DEBUG] Reactor Error (Post-Response): parsed 0:0:0:0. Keeping previous mindstate %s\n", c.GetMindState())
						}
					} else if c.DebugMode {
						fmt.Fprintf(c.OutWriter, "[DEBUG] Reactor Error (Post-Response): %v\n", err)
					}
				}

				// Ensure at least 3 seconds have passed
				time.Sleep(time.Until(startTime.Add(3 * time.Second)))
				done <- true
				
				// Save assistant response to long-term history and responder STM
				_ = c.HistoryMgr.Save(c.PersonalityName, reply, c.GetMindState())
				c.ResponderSTM.Update("assistant", reply)
				c.ReactorSTM.Update("assistant", reply)

				if rawInput.ResponseChan != nil {
					rawInput.ResponseChan <- reply
				}

				for _, line := range strings.Split(reply, "\n") {
					if strings.TrimSpace(line) != "" {
						fmt.Fprintf(c.OutWriter, "\033[34m> %s\033[0m\n", line)
					}
				}
				c.Sched.Engine.OnResponse()
			}
		}
	}
}
