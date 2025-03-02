package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *sql.DB
	platform       string
}

type user struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type userRequest struct {
	Email string `json:"email"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: Could not load .env file")
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		fmt.Println("Error: DB_URL not set in environment")
		return
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("Error connecting to database:", err)
		return
	}
	defer db.Close()

	platform := os.Getenv("PLATFORM")

	mux := http.NewServeMux()
	apiCfg := &apiConfig{db: db, platform: platform}

	mux.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)

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

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req userRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	id := uuid.New().String()
	timestamp := time.Now().UTC()

	_, err := cfg.db.Exec("INSERT INTO users (id, email, created_at, updated_at) VALUES ($1, $2, $3, $4)", id, req.Email, timestamp, timestamp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create user"})
		return
	}

	resp := user{ID: id, Email: req.Email, CreatedAt: timestamp, UpdatedAt: timestamp}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden"})
		return
	}

	_, err := cfg.db.Exec("DELETE FROM users")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to reset users"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "All users deleted"})
}
