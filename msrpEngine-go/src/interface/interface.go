package engineInterface

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"msrpengine/src/agents/reactor"
	"msrpengine/src/agents/responder"
	
	"msrpengine/src/escalator"
	"msrpengine/src/idle_methods/episode_memory"
	"msrpengine/src/interface/api"
	"msrpengine/src/contextManager"
	"msrpengine/src/utils"
)

type AppCore struct {
	HistoryMgr      *contextManager.EventLogContext
	IndexMgr        *contextManager.ChromemIndexManager
	EpisodeMgr      *episode_memory.EpisodeMemoryManager
	ReactorSTM      *contextManager.ShortTermContext
	ResponderSTM    *contextManager.ShortTermContext
	Sched           *escalator.Scheduler
	Resp            *responder.Responder
	ReactorAgent    *reactor.ReactorAgent
	OllamaCmd       *exec.Cmd

	MindStateVal         string
	HasUnconsolidatedVal bool
	UnconsolidatedChars  int
	isConsolidating      bool
	StateMu              sync.RWMutex
	
	DebugMode       bool
	PersonalityName string
	OutWriter       io.Writer
	
	InputQueue      chan api.ChatInput
	
	UnreactedChars  int
	
	MaxInputChars   int
	MaxOutputChars  int
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

func (c *AppCore) GetUnconsolidatedChars() int {
	c.StateMu.RLock()
	defer c.StateMu.RUnlock()
	return c.UnconsolidatedChars
}

func (c *AppCore) AddUnconsolidatedChars(n int) {
	c.StateMu.Lock()
	defer c.StateMu.Unlock()
	c.UnconsolidatedChars += n
}

func (c *AppCore) ResetUnconsolidatedChars() {
	c.StateMu.Lock()
	defer c.StateMu.Unlock()
	c.UnconsolidatedChars = 0
}

func (c *AppCore) StartConsolidation() bool {
	c.StateMu.Lock()
	defer c.StateMu.Unlock()
	if c.isConsolidating {
		return false
	}
	c.isConsolidating = true
	return true
}

func (c *AppCore) FinishConsolidation() {
	c.StateMu.Lock()
	defer c.StateMu.Unlock()
	c.isConsolidating = false
}

func (c *AppCore) GetCurrentMetrics() contextManager.Metrics {
	if c.Sched == nil || c.Sched.Engine == nil {
		return contextManager.Metrics{
			EnergyLevel:     800.0,
			EnergyDrainRate: 10.0,
			MindScores:      c.GetMindState(),
		}
	}
	return contextManager.Metrics{
		EnergyLevel:     c.Sched.Engine.GetMentalEnergy(),
		EnergyDrainRate: c.Sched.Engine.GetEnergyDrainRate(),
		MindScores:      c.GetMindState(),
	}
}

func (c *AppCore) InjectSystemMessage(sysMsg string) {
	_ = c.HistoryMgr.Save("system", sysMsg, c.GetCurrentMetrics())
	c.ReactorSTM.Update("system", sysMsg)
	c.ResponderSTM.Update("system", sysMsg)
}

func (c *AppCore) Shutdown() {
	if c.OllamaCmd != nil && c.OllamaCmd.Process != nil {
		if c.DebugMode {
			fmt.Println("system: gracefully shutting down local embedding engine...")
		}
		c.OllamaCmd.Process.Kill()
	}
}

type readliner interface {
	Close() error
	Stdout() io.Writer
	SetPrompt(string)
	Refresh()
}

// Run starts the interactive chat interface for Lyra.
func Run(newSession bool, reuseSession string, debugMode bool, serverMode bool) {
	core, err := NewAppCore(newSession, reuseSession, debugMode)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer core.Shutdown()

	// Initialize Readline
	historyDir := utils.ResolvePath(filepath.Join("Context", "interfaceEventLog"))
	var outWriter io.Writer = os.Stdout
	var rl readliner
	inputChan := make(chan string)
	
	if !serverMode {
		var err error
		var rlInst readlinerImpl
		rlInst, err = StartCLI(historyDir, inputChan)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		rl = rlInst
		if rl != nil {
			defer rl.Close()
			outWriter = rlInst.Stdout()
		}
	} else {
		// No readline, everything just uses standard outWriter = os.Stdout
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
	go api.StartServer(apiInputChan, core.HistoryMgr, core.Sched, core.GetMindState)

	core.OutWriter = outWriter
	core.InputQueue = processChan
	core.RunLoop(engineStartTime, lastWakeTime, rl)
}
