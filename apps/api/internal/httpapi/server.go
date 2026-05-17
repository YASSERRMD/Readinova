// Package httpapi implements the REST API server.
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/YASSERRMD/Readinova/apps/api/internal/billing"
	"github.com/YASSERRMD/Readinova/apps/api/internal/platform/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server holds shared dependencies.
type Server struct {
	db        *pgxpool.Pool
	jwtSecret []byte
}

// New creates a new Server.  It panics if jwtSecret is shorter than 32 bytes
// to catch misconfiguration at startup rather than at runtime.
func New(db *pgxpool.Pool, jwtSecret []byte) *Server {
	if len(jwtSecret) < 32 {
		panic(fmt.Sprintf("httpapi: jwtSecret must be at least 32 bytes, got %d", len(jwtSecret)))
	}
	return &Server{db: db, jwtSecret: jwtSecret}
}

// Handler returns an http.Handler wrapping all routes with telemetry middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.Routes(mux)
	return telemetry.Middleware(mux)
}

// Routes registers all API routes on the given mux.
func (s *Server) Routes(mux *http.ServeMux) {
	// Metrics endpoint (no auth required; scrape from internal network only).
	mux.Handle("GET /metrics", telemetry.MetricsHandler())
	// Auth (rate-limited to prevent brute-force).
	mux.HandleFunc("POST /v1/organisations", withAuthRateLimit(s.handleSignup))
	mux.HandleFunc("POST /v1/auth/login", withAuthRateLimit(s.handleLogin))
	mux.HandleFunc("POST /v1/auth/refresh", s.handleRefresh)
	mux.HandleFunc("POST /v1/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /v1/me", s.withAuth(s.handleMe))

	// Org management
	mux.HandleFunc("PATCH /v1/organisations/{id}", s.withAuth(s.handlePatchOrg))
	mux.HandleFunc("GET /v1/members", s.withAuth(s.handleListMembers))

	// Invitations
	mux.HandleFunc("POST /v1/invitations", s.withAuth(s.handleCreateInvitation))
	mux.HandleFunc("POST /v1/invitations/{token}/accept", withAuthRateLimit(s.handleAcceptInvitation))

	// Assessments
	s.assessmentRoutes(mux)

	// Responses
	s.responseRoutes(mux)

	// Scoring
	s.scoringRoutes(mux)

	// Reports
	s.reportRoutes(mux)

	// Billing
	s.billingRoutes(mux)

	// Evidence Connectors
	s.connectorRoutes(mux)

	// Perception Gap
	s.perceptionRoutes(mux)

	// Recommendations
	s.recommendRoutes(mux)

	// Audit Artefacts
	s.artefactRoutes(mux)
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
// It limits the body to 1 MiB to prevent unbounded memory consumption.
func decodeJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1 MiB
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// dimLabelFromSlug converts a snake_case dimension slug to a title-case label
// (e.g. "data_governance" → "Data governance").
func dimLabelFromSlug(slug string) string {
	label := strings.ReplaceAll(slug, "_", " ")
	if len(label) == 0 {
		return label
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

// tierFor returns the billing tier for an organisation, defaulting to TierFree
// when no active subscription exists.  Use this instead of duplicating the
// subscription query across handler files.
func (s *Server) tierFor(ctx context.Context, orgID string) billing.Tier {
	var tier string
	_ = s.db.QueryRow(ctx,
		`SELECT tier FROM subscriptions WHERE organisation_id = $1`, orgID,
	).Scan(&tier)
	if tier == "" {
		return billing.TierFree
	}
	return billing.Tier(tier)
}
