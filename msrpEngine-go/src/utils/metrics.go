package utils

import (
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	metricsMu sync.Mutex
	totalSent int
	totalRecv int
)

// LogMetrics logs the number of characters sent and received in an API request.
func LogMetrics(agent string, sent int, recv int) {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	totalSent += sent
	totalRecv += recv

	filename := "api_metrics.csv"
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[DEBUG] Failed to open metrics csv: %v\n", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// If file is empty, write header
	stat, _ := file.Stat()
	if stat.Size() == 0 {
		_ = writer.Write([]string{"Timestamp", "Agent", "SentChars", "ReceivedChars", "TotalSentChars", "TotalReceivedChars"})
	}

	record := []string{
		time.Now().Format(time.RFC3339),
		agent,
		fmt.Sprintf("%d", sent),
		fmt.Sprintf("%d", recv),
		fmt.Sprintf("%d", totalSent),
		fmt.Sprintf("%d", totalRecv),
	}
	_ = writer.Write(record)
	
	LogDebug("Metrics [%s] - Sent: %d chars | Recv: %d chars | Total Sent: %d | Total Recv: %d", agent, sent, recv, totalSent, totalRecv)
}
