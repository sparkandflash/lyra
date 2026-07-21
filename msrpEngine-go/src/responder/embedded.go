package responder

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"msrpengine/src/consolidator"
)

//go:embed models/default.gguf
var embeddedModelData []byte

type EmbeddedResponder struct {
	config Config
	runner *LocalBinaryResponder
}

func NewEmbeddedResponder(config Config) (*EmbeddedResponder, error) {
	// If the embedded file is just our tiny placeholder (or empty), return an error explaining how to set it up.
	if len(embeddedModelData) < 1024 {
		return nil, fmt.Errorf("embedded model is missing or invalid. Please replace the placeholder file at 'responder/models/default.gguf' with a real GGUF model and recompile Lyra")
	}

	// Write embedded data to a temporary file for the local binary runner to mmap.
	tempDir := os.TempDir()
	tempModelPath := filepath.Join(tempDir, "lyra_embedded_model.gguf")

	// We only write the file if it doesn't exist or is of a different size to minimize startup overhead.
	info, err := os.Stat(tempModelPath)
	if err != nil || info.Size() != int64(len(embeddedModelData)) {
		err = os.WriteFile(tempModelPath, embeddedModelData, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to write embedded model to temp disk: %w", err)
		}
	}

	// Update the config model path to point to our temp file and use the local binary runner.
	config.Model = tempModelPath
	runner := NewLocalBinaryResponder(config)

	return &EmbeddedResponder{
		config: config,
		runner: runner,
	}, nil
}

func (r *EmbeddedResponder) Respond(ctx context.Context, prompt string, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (string, string, error) {
	return r.runner.Respond(ctx, prompt, mindState, history, episodes)
}

func (r *EmbeddedResponder) RespondProactive(ctx context.Context, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (string, string, error) {
	return r.runner.RespondProactive(ctx, mindState, history, episodes)
}
