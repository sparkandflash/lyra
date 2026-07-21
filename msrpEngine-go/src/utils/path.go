package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// GetBasePath returns the absolute directory path where the executable is located.
// If run via 'go run' (which builds a temporary binary in /tmp), it falls back to the current working directory.
func GetBasePath() string {
	execPath, err := os.Executable()
	if err != nil {
		pwd, _ := os.Getwd()
		return pwd
	}

	// Check if this is a temporary go run binary
	if strings.Contains(execPath, "go-build") || strings.Contains(execPath, "/var/folders/") || strings.Contains(execPath, "/tmp/") {
		pwd, _ := os.Getwd()
		return pwd
	}

	return filepath.Dir(execPath)
}

// ResolvePath takes a relative path (like ".env" or "Context") and resolves it relative to the binary's location.
func ResolvePath(relPath string) string {
	if filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Join(GetBasePath(), relPath)
}
