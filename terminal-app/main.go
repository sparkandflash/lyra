package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"terminal-app/src/interface"
	"terminal-app/src/responder"
	"terminal-app/src/utils"

	"github.com/joho/godotenv"
)

func main() {
	newSession := flag.Bool("newSession", false, "Start a fresh session ignoring previous history")
	reuseSession := flag.String("reuseSession", "", "Reuse a specific session ID")
	debug := flag.Bool("debug", false, "Run in debug mode with verbose logging")
	flag.Parse()

	// Load .env relative to the executable path for standalone support
	_ = godotenv.Load(utils.ResolvePath(".env"))

	fmt.Println("Initializing systems. Validating agent credentials...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Validate Responder
	respCfg := responder.LoadConfigFromEnv()
	if err := responder.ValidateConfig(ctx, respCfg); err != nil {
		fmt.Printf("\033[31m[FATAL] Responder Agent failed validation:\033[0m %v\n", err)
		os.Exit(1)
	}

	// Validate Reactor
	reactCfg := responder.LoadReactorConfigFromEnv()
	if err := responder.ValidateConfig(ctx, reactCfg); err != nil {
		fmt.Printf("\033[31m[FATAL] Reactor Agent failed validation:\033[0m %v\n", err)
		os.Exit(1)
	}

	// Validate Summariser
	sumCfg := responder.LoadSummariserConfigFromEnv()
	if err := responder.ValidateConfig(ctx, sumCfg); err != nil {
		fmt.Printf("\033[31m[FATAL] Summariser Agent failed validation:\033[0m %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\033[32mAll agents validated successfully. Starting chat...\033[0m")
	cli.Run(*newSession, *reuseSession, *debug)
}

