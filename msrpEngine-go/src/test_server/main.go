package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Rate Limiter to simulate Gemini Free Tier (15 RPM)
var (
	requestTimes []time.Time
	rlMu         sync.Mutex
)

func checkRateLimit() bool {
	rlMu.Lock()
	defer rlMu.Unlock()

	now := time.Now()
	// Filter out requests older than 1 minute
	validTimes := []time.Time{}
	for _, t := range requestTimes {
		if now.Sub(t) < time.Minute {
			validTimes = append(validTimes, t)
		}
	}
	requestTimes = validTimes

	// Check against 15 RPM limit
	if len(requestTimes) >= 15 {
		return false
	}
	
	requestTimes = append(requestTimes, now)
	return true
}

// OpenAI compatible structs
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

// Gibberish words for fake text generation
var words = []string{"foo", "bar", "baz", "qux", "zap", "zop", "flibber", "blabber", "wobble", "jibber", "jabber"}

func randomText(wordCount int) string {
	var result []string
	for i := 0; i < wordCount; i++ {
		result = append(result, words[rand.Intn(len(words))])
	}
	return strings.Join(result, " ")
}

func randomFloat() float64 {
	// Returns a float between -1.0 and 1.0
	return (rand.Float64() * 2.0) - 1.0
}

func handleCompletions(w http.ResponseWriter, r *http.Request) {
	if !checkRateLimit() {
		http.Error(w, "Too Many Requests (Simulated 429)", http.StatusTooManyRequests)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req openAIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var content string

	modelName := strings.ToLower(req.Model)

	if strings.Contains(modelName, "reactor") {
		// Mock Reactor Response (Floats between -1.0 and 1.0)
		content = fmt.Sprintf(`{
			"model_attention": %.2f,
			"user_attention": %.2f,
			"serotonin": %.2f,
			"oxytocin": %.2f,
			"cortisol": %.2f
		}`, randomFloat(), randomFloat(), randomFloat(), randomFloat(), randomFloat())
	} else if strings.Contains(modelName, "responder") {
		// Mock Responder Response
		content = fmt.Sprintf(`{
			"reply": "%s",
			"useful_episode_id": ""
		}`, randomText(15))
	} else if strings.Contains(modelName, "summariser") || strings.Contains(modelName, "consolidation") {
		// Mock Summariser Response
		content = fmt.Sprintf(`[
			{
				"id": "ep_%d",
				"facts": "%s"
			}
		]`, time.Now().Unix(), randomText(8))
	} else {
		// Generic gibberish fallback
		content = randomText(10)
	}

	// Wait 200ms to mock slight network/processing delay
	time.Sleep(200 * time.Millisecond)

	resp := openAIResponse{}
	resp.Choices = append(resp.Choices, struct {
		Message openAIMessage `json:"message"`
	}{
		Message: openAIMessage{
			Role:    "assistant",
			Content: content,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if !checkRateLimit() {
		http.Error(w, "Too Many Requests (Simulated 429)", http.StatusTooManyRequests)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Create a random embedding of length 768
	var vector []float32
	for i := 0; i < 768; i++ {
		vector = append(vector, float32((rand.Float64()*2.0)-1.0))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"embeddings": [][]float32{vector},
	})
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"data": [{"id": "mock-model"}]}`))
}

func main() {
	rand.Seed(time.Now().UnixNano())

	mux := http.NewServeMux()
	// Catch-all or specific paths for OpenAI compatibility
	mux.HandleFunc("/chat/completions", handleCompletions)
	mux.HandleFunc("/v1/chat/completions", handleCompletions)
	mux.HandleFunc("/v1/embeddings", handleEmbeddings)
	mux.HandleFunc("/embeddings", handleEmbeddings)
	mux.HandleFunc("/v1/models", handleModels)
	mux.HandleFunc("/models", handleModels)

	port := "8081"
	fmt.Printf("\033[36m[Mock LLM Server running on port %s]\033[0m\n", port)
	
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
