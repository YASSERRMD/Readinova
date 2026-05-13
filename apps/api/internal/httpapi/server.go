// Package httpapi implements the REST API server.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Server holds shared dependencies.
type Server struct {
	db        *pgxpool.Pool
	jwtSecret []byte
}

// New creates a new Server.
func New(db *pgxpool.Pool, jwtSecret []byte) *Server {
	return &Server{db: db, jwtSecret: jwtSecret}
}

// Routes registers all API routes on the given mux.
func (s *Server) Routes(mux *http.ServeMux) {
	// Auth
	mux.HandleFunc("POST /v1/organisations", s.handleSignup)
	mux.HandleFunc("POST /v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /v1/auth/refresh", s.handleRefresh)
	mux.HandleFunc("GET /v1/me", s.withAuth(s.handleMe))

	// Org management
	mux.HandleFunc("PATCH /v1/organisations/{id}", s.withAuth(s.handlePatchOrg))

	// Invitations
	mux.HandleFunc("POST /v1/invitations", s.withAuth(s.handleCreateInvitation))
	mux.HandleFunc("POST /v1/invitations/{token}/accept", s.handleAcceptInvitation)
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeJSON reads and decodes the request body into v.
func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
