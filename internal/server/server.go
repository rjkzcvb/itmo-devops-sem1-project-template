package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type Server struct {
	db *sql.DB
}

func NewServer(db *sql.DB) *Server {
	return &Server{db: db}
}

func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	
	mux.HandleFunc("POST /api/v0/prices", s.UploadPrices)
	mux.HandleFunc("GET /api/v0/prices", s.DownloadPrices)
	mux.HandleFunc("GET /health", s.HealthCheck)
	
	return mux
}

func (s *Server) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":   "ok",
		"database": "connected",
	}
	writeJSON(w, http.StatusOK, response)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
