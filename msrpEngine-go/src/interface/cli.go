package engineInterface

import (
	"fmt"
	"strings"

	"github.com/chzyer/readline"
)

type readlinerImpl interface {
	Close() error
	Readline() (string, error)
}

func StartCLI(historyDir string, inputChan chan<- string) (readlinerImpl, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:      "> ",
		HistoryFile: historyDir + "/readline_history.txt",
	})
	if err != nil {
		return nil, fmt.Errorf("system error: failed to initialize readline: %v", err)
	}

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

	return rl, nil
}
