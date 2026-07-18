package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"terminal-app/src/consolidator"
	"terminal-app/src/escalator"
)

var (
	jwtSecret = []byte(getEnv("JWT_SECRET", "supersecret"))
	webUser   = getEnv("WEB_USER", "admin")
	webPass   = getEnv("WEB_PASS", "password")
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// ChatInput represents a user message and a channel to return the LLM's response.
type ChatInput struct {
	Message      string
	ResponseChan chan string
}

// Server handles the web API.
type Server struct {
	InputChan     chan<- ChatInput
	HistoryMgr    *consolidator.HistoryManager
	Sched         *escalator.Scheduler
	MindStateFunc func() string
}

// StartServer initializes and starts the HTTP server on port 8080.
func StartServer(inputChan chan<- ChatInput, historyMgr *consolidator.HistoryManager, sched *escalator.Scheduler, msFunc func() string) {
	s := &Server{
		InputChan:     inputChan,
		HistoryMgr:    historyMgr,
		Sched:         sched,
		MindStateFunc: msFunc,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/getMessages", s.authMiddleware(s.handleGetMessages))
	mux.HandleFunc("/sendMessage", s.authMiddleware(s.handleSendMessage))

	port := getEnv("PORT", "8080")
	fmt.Printf("\033[36m[System: Web API started on port %s]\033[0m\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Printf("Web API Error: %v\n", err)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if creds.Username != webUser || creds.Password != webPass {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": creds.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	lastID := r.URL.Query().Get("last_id")
	allMsgs := s.HistoryMgr.GetMessages()
	
	var newMsgs []consolidator.Message
	for _, msg := range allMsgs {
		if lastID == "" || msg.ID > lastID {
			newMsgs = append(newMsgs, msg)
		}
	}

	// Prepare physiological state
	hr := s.Sched.Engine.GetHeartrate()
	energy := s.Sched.Engine.GetMentalEnergy()
	mindStateStr := s.MindStateFunc()

	response := map[string]interface{}{
		"messages":      newMsgs,
		"heartrate":     hr,
		"mental_energy": energy,
		"mind_state":    mindStateStr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	msg := strings.TrimSpace(payload.Message)
	if msg == "" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"reply": ""})
		return
	}

	respChan := make(chan string, 1)
	s.InputChan <- ChatInput{
		Message:      msg,
		ResponseChan: respChan,
	}

	select {
	case reply := <-respChan:
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"reply": reply})
	case <-time.After(15 * time.Second):
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}
