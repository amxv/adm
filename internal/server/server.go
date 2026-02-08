package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
)

// Server holds the HTTP server state.
type Server struct {
	db   *sql.DB
	mux  *http.ServeMux
	addr string
}

// New creates a new Server bound to the given address.
func New(db *sql.DB, host string, port int) *Server {
	s := &Server{
		db:   db,
		addr: fmt.Sprintf("%s:%d", host, port),
	}
	s.mux = http.NewServeMux()
	s.registerRoutes()
	return s
}

// Addr returns the bind address.
func (s *Server) Addr() string {
	return s.addr
}

// Handler returns the HTTP handler for use in http.Server or testing.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/messages", s.handleMessages)
	s.mux.HandleFunc("GET /api/v1/messages/{id}", s.handleMessageDetail)
	s.mux.HandleFunc("GET /api/v1/agents", s.handleAgents)
	s.mux.HandleFunc("GET /api/v1/claims", s.handleClaims)
	s.mux.HandleFunc("GET /api/v1/claims/conflicts", s.handleClaimConflicts)
	s.mux.HandleFunc("GET /api/v1/debug/delivery", s.handleDeliveryDebug)
	s.mux.HandleFunc("GET /api/v1/audit", s.handleAuditLog)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
