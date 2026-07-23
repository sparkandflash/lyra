package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	isServerMode bool
	isDebugMode  bool
	logFileMu    sync.Mutex
	outWriter    io.Writer = os.Stdout
)

// SetRunMode configures the logger's console output behavior.
func SetRunMode(server, debug bool) {
	isServerMode = server
	isDebugMode = debug
}

// SetOutWriter sets the writer for console output (useful for Readline).
func SetOutWriter(w io.Writer) {
	outWriter = w
}

// LogDebug writes telemetry to logs.txt always.
// If isServerMode or isDebugMode is true, it also prints to stdout in ANSI grey.
func LogDebug(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	
	// Ensure it ends with a newline for file logging
	fileMsg := msg
	if len(fileMsg) > 0 && fileMsg[len(fileMsg)-1] != '\n' {
		fileMsg += "\n"
	}
	
	timestampedMsg := fmt.Sprintf("[%s] %s", time.Now().Format(time.RFC3339), fileMsg)

	// Write to logs.txt
	logFileMu.Lock()
	logPath := ResolvePath(filepath.Join("logs.txt"))
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		_, _ = f.WriteString(timestampedMsg)
		f.Close()
	}
	logFileMu.Unlock()

	// Print to console if in debug or server mode
	if isDebugMode || isServerMode {
		// ANSI Grey text
		fmt.Fprintf(outWriter, "\033[90m[DEBUG] %s\033[0m\n", msg)
	}
}
