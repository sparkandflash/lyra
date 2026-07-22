package contextManager

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"msrpengine/src/utils"
)

// ResolveSession resolves the sessionID, mindState, and mentalEnergy from the sessions.csv ledger.
// If newSession is true, it returns empty/default values so a new session is generated.
// If reuseSession is not empty, it attempts to load that specific session.
// Otherwise, it attempts to load the most recent session.
func ResolveSession(newSession bool, reuseSession string) (sessionID string, savedMindState string, savedMentalEnergy float64) {
	savedMentalEnergy = 800.0 // Default mental energy

	if newSession {
		sessionID = ""
		return
	}

	historyDir := utils.ResolvePath(filepath.Join("Context", "interfaceEventLog"))
	csvPath := filepath.Join(historyDir, "sessions.csv")

	file, err := os.Open(csvPath)
	if err != nil {
		if reuseSession != "" {
			sessionID = reuseSession
		}
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, _ := reader.ReadAll()

	if reuseSession != "" {
		sessionID = reuseSession
		for i := len(records) - 1; i >= 1; i-- {
			if len(records[i]) >= 3 && records[i][0] == sessionID {
				savedMindState = records[i][1]
				fmt.Sscanf(records[i][2], "%f", &savedMentalEnergy)
				break
			}
		}
	} else {
		if len(records) > 1 {
			lastRow := records[len(records)-1]
			if len(lastRow) >= 3 {
				sessionID = lastRow[0]
				savedMindState = lastRow[1]
				fmt.Sscanf(lastRow[2], "%f", &savedMentalEnergy)
			}
		}
	}

	return
}

// UpdateSessionCSV updates or appends the session ledger with the given state.
func UpdateSessionCSV(sessionID, mindState string, mentalEnergy float64) {
	historyDir := utils.ResolvePath(filepath.Join("Context", "interfaceEventLog"))
	os.MkdirAll(historyDir, 0755)
	
	csvPath := filepath.Join(historyDir, "sessions.csv")
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
