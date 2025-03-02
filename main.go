package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *sql.DB
}

var forbiddenWords = []string{"kerfuffle", "sharbert", "fornax"}

const maxChirpLength = 140

type chirpRequest struct {
	Body string `json:"body"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type successResponse struct {
	CleanedBody string `json:"cleaned_body"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: Could not load .env file")
	}

	// Get DB connection string
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		fmt.Println("Error: DB_URL not set in environment")
		return
	}

	// Open database connection
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("Error connecting to database:", err)
		return
	}
	defer db.Close()

	mux := http.NewServeMux()
	apiCfg := &apiConfig{db: db}

	mux.HandleFunc("GET /api/healthz", readinessHandler)

	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", fileServer)))

	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)

	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)

	mux.Handle("/assets/logo.png", fileServer)

	mux.HandleFunc("POST /api/validate_chirp", chirpValidateHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	fmt.Println("Starting server on :8080...")
	err = server.ListenAndServe()
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func chirpValidateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req chirpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		resp, _ := json.Marshal(errorResponse{Error: "Invalid request body"})
		w.Write(resp)
		return
	}

	if len(req.Body) > maxChirpLength {
		w.WriteHeader(http.StatusBadRequest)
		resp, _ := json.Marshal(errorResponse{Error: "Chirp is too long"})
		w.Write(resp)
		return
	}

	cleanedBody := censorText(req.Body, forbiddenWords)

	w.WriteHeader(http.StatusOK)
	resp, _ := json.Marshal(successResponse{CleanedBody: cleanedBody})
	w.Write(resp)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func censorText(text string, words []string) string {
	wordsInText := strings.Split(text, " ")

	for i, word := range wordsInText {
		lowerWord := strings.ToLower(word)
		for _, forbidden := range words {
			if lowerWord == forbidden {
				wordsInText[i] = "****"
				break
			}
		}
	}

	return strings.Join(wordsInText, " ")
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	count := cfg.fileserverHits.Load()
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
  <head>
    <title>Chirpy Metrics</title>
  </head>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, count)
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Counter reset\n"))
}
