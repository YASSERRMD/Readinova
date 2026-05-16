package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/YASSERRMD/Readinova/apps/api/internal/billing"
	"github.com/YASSERRMD/Readinova/apps/api/internal/connector"
)

// connectorRoutes adds evidence connector endpoints to the mux.
func (s *Server) connectorRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/connectors", s.withAuth(s.handleListConnectors))
	mux.HandleFunc("PUT /v1/connectors/{type}", s.withAuth(s.handleUpsertConnector))
	mux.HandleFunc("DELETE /v1/connectors/{type}", s.withAuth(s.handleDeleteConnector))
	mux.HandleFunc("POST /v1/connectors/{type}/sync", s.withAuth(s.handleSyncConnector))
	mux.HandleFunc("GET /v1/evidence", s.withAuth(s.handleListEvidence))
}

// GET /v1/connectors — list all connector configs for the org.
func (s *Server) handleListConnectors(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)

	if !s.hasEvidenceConnectors(r, claims.OrgID) {
		writeError(w, http.StatusPaymentRequired, "evidence connectors require Growth tier or above")
		return
	}

	rows, err := s.db.Query(r.Context(),
		`SELECT connector_type, display_name, enabled, last_sync_at, last_sync_error, created_at
		 FROM connector_configs WHERE organisation_id = $1 ORDER BY created_at`,
		claims.OrgID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	type row struct {
		ConnectorType string  `json:"connector_type"`
		DisplayName   string  `json:"display_name"`
		Enabled       bool    `json:"enabled"`
		LastSyncAt    *string `json:"last_sync_at,omitempty"`
		LastSyncError *string `json:"last_sync_error,omitempty"`
		CreatedAt     string  `json:"created_at"`
	}
	var list []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ConnectorType, &x.DisplayName, &x.Enabled, &x.LastSyncAt, &x.LastSyncError, &x.CreatedAt); err != nil {
			continue
		}
		list = append(list, x)
	}
	if list == nil {
		list = []row{}
	}
	writeJSON(w, http.StatusOK, list)
}

// PUT /v1/connectors/{type} — upsert a connector config (owner only).
func (s *Server) handleUpsertConnector(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims.Role != "owner" && claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "only owner or admin may manage connectors")
		return
	}
	if !s.hasEvidenceConnectors(r, claims.OrgID) {
		writeError(w, http.StatusPaymentRequired, "evidence connectors require Growth tier or above")
		return
	}

	connType := r.PathValue("type")
	if _, ok := connector.Registry[connType]; !ok {
		writeError(w, http.StatusBadRequest, "unknown connector type")
		return
	}

	var req struct {
		DisplayName string         `json:"display_name"`
		Credentials map[string]any `json:"credentials"`
		Enabled     *bool          `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = connType
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	credJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid credentials")
		return
	}

	_, err = s.db.Exec(r.Context(),
		`INSERT INTO connector_configs (organisation_id, connector_type, display_name, credentials, enabled)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (organisation_id, connector_type)
		 DO UPDATE SET display_name=$3, credentials=$4, enabled=$5, updated_at=now()`,
		claims.OrgID, connType, req.DisplayName, credJSON, enabled,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"connector_type": connType, "status": "saved"})
}

// DELETE /v1/connectors/{type} — remove a connector config.
func (s *Server) handleDeleteConnector(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims.Role != "owner" && claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "only owner or admin may manage connectors")
		return
	}

	connType := r.PathValue("type")
	tag, err := s.db.Exec(r.Context(),
		`DELETE FROM connector_configs WHERE organisation_id = $1 AND connector_type = $2`,
		claims.OrgID, connType,
	)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "connector not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/connectors/{type}/sync — run a sync and persist signals.
func (s *Server) handleSyncConnector(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if !s.hasEvidenceConnectors(r, claims.OrgID) {
		writeError(w, http.StatusPaymentRequired, "evidence connectors require Growth tier or above")
		return
	}

	connType := r.PathValue("type")
	factory, ok := connector.Registry[connType]
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown connector type")
		return
	}

	var credJSON []byte
	if err := s.db.QueryRow(r.Context(),
		`SELECT credentials FROM connector_configs WHERE organisation_id = $1 AND connector_type = $2 AND enabled = true`,
		claims.OrgID, connType,
	).Scan(&credJSON); err != nil {
		writeError(w, http.StatusNotFound, "connector not configured or disabled")
		return
	}

	var creds map[string]any
	_ = json.Unmarshal(credJSON, &creds)

	conn := factory()
	if err := conn.Connect(r.Context(), creds); err != nil {
		s.markSyncError(r, claims.OrgID, connType, err.Error())
		writeError(w, http.StatusBadGateway, "connector connect failed: "+err.Error())
		return
	}
	defer conn.Disconnect(r.Context()) //nolint:errcheck

	signals, err := conn.Collect(r.Context(), nil)
	if err != nil {
		s.markSyncError(r, claims.OrgID, connType, err.Error())
		writeError(w, http.StatusBadGateway, "connector collect failed: "+err.Error())
		return
	}

	// Persist signals in a single transaction.
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Upsert signals so repeated syncs update existing rows rather than creating
	// an ever-growing append-only history.
	for _, sig := range signals {
		valJSON, _ := json.Marshal(sig.SignalValue)
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO evidence_signals
			   (organisation_id, connector_type, dimension_slug, signal_key, signal_value)
			 VALUES ($1,$2,$3,$4,$5)
			 ON CONFLICT (organisation_id, connector_type, signal_key)
			 DO UPDATE SET signal_value = EXCLUDED.signal_value, collected_at = now()`,
			claims.OrgID, connType, sig.DimensionSlug, sig.SignalKey, valJSON,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	// Update last_sync_at.
	_, _ = s.db.Exec(r.Context(),
		`UPDATE connector_configs SET last_sync_at=now(), last_sync_error=NULL, updated_at=now()
		 WHERE organisation_id=$1 AND connector_type=$2`,
		claims.OrgID, connType,
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"connector_type": connType,
		"signals_synced": len(signals),
	})
}

// GET /v1/evidence — list recent evidence signals for the org.
func (s *Server) handleListEvidence(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if !s.hasEvidenceConnectors(r, claims.OrgID) {
		writeError(w, http.StatusPaymentRequired, "evidence connectors require Growth tier or above")
		return
	}

	dimFilter := r.URL.Query().Get("dimension")
	query := `SELECT connector_type, dimension_slug, signal_key, signal_value, collected_at
	           FROM evidence_signals WHERE organisation_id = $1`
	args := []any{claims.OrgID}
	if dimFilter != "" {
		query += " AND dimension_slug = $2"
		args = append(args, dimFilter)
	}
	query += " ORDER BY collected_at DESC LIMIT 200"

	rows, err := s.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	type row struct {
		ConnectorType string          `json:"connector_type"`
		DimensionSlug string          `json:"dimension_slug"`
		SignalKey     string          `json:"signal_key"`
		SignalValue   json.RawMessage `json:"signal_value"`
		CollectedAt   string          `json:"collected_at"`
	}
	var list []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ConnectorType, &x.DimensionSlug, &x.SignalKey, &x.SignalValue, &x.CollectedAt); err != nil {
			continue
		}
		list = append(list, x)
	}
	if list == nil {
		list = []row{}
	}
	writeJSON(w, http.StatusOK, list)
}

// hasEvidenceConnectors checks whether the org's tier allows evidence connectors.
func (s *Server) hasEvidenceConnectors(r *http.Request, orgID string) bool {
	return billing.LimitsFor(s.tierFor(r.Context(), orgID)).EvidenceConnectors
}

// markSyncError records a sync error on the connector config.
func (s *Server) markSyncError(r *http.Request, orgID, connType, errMsg string) {
	_, _ = s.db.Exec(r.Context(),
		`UPDATE connector_configs SET last_sync_error=$1, updated_at=now()
		 WHERE organisation_id=$2 AND connector_type=$3`,
		errMsg, orgID, connType,
	)
}
