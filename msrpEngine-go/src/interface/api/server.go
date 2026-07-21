package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"msrpengine/src/consolidator"
	"msrpengine/src/escalator"
)

// (Globals removed because they evaluated before main() loaded .env)

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
	mux.HandleFunc("/", s.handleHealth)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/getMessages", s.authMiddleware(s.handleGetMessages))
	mux.HandleFunc("/getMessageHistory", s.authMiddleware(s.handleGetMessageHistory))
	mux.HandleFunc("/sendMessage", s.authMiddleware(s.handleSendMessage))

	port := getEnv("PORT", "8080")
	if getEnv("DEBUG", "0") == "1" || getEnv("DEBUG", "false") == "true" {
		fmt.Printf("\033[36m[System: Web API started on port %s]\033[0m\n", port)
	}
	if err := http.ListenAndServe(":"+port, s.corsMiddleware(mux)); err != nil {
		fmt.Printf("Web API Error: %v\n", err)
	}
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := getEnv("CORS_ORIGIN", "*")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// If it's a preflight OPTIONS request, stop here and return 200
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "online",
		"engine": "lyra",
		"time":   time.Now().Format(time.RFC3339),
	})
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

	webUser := getEnv("WEB_USER", "admin")
	webPass := getEnv("WEB_PASS", "password")
	jwtSecret := []byte(getEnv("JWT_SECRET", "supersecret"))

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
		jwtSecret := []byte(getEnv("JWT_SECRET", "supersecret"))
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
	case <-time.After(90 * time.Second):
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}

func (s *Server) handleGetMessageHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allMsgs := s.HistoryMgr.GetMessages()
	
	offsetStr := r.URL.Query().Get("offset")
	lengthStr := r.URL.Query().Get("length")
	
	offset := 0
	length := len(allMsgs)
	
	if offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil && val >= 0 {
			offset = val
		}
	}
	if lengthStr != "" {
		if val, err := strconv.Atoi(lengthStr); err == nil && val > 0 {
			length = val
		}
	}
	
	if offset > len(allMsgs) {
		offset = len(allMsgs)
	}
	
	end := offset + length
	if end > len(allMsgs) {
		end = len(allMsgs)
	}
	
	var paginatedMsgs []consolidator.Message
	if offset < end {
		paginatedMsgs = allMsgs[offset:end]
	} else {
		paginatedMsgs = []consolidator.Message{}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": paginatedMsgs,
		"total":    len(allMsgs),
	})
}
