package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/YASSERRMD/Readinova/apps/api/internal/artefact"
	"github.com/YASSERRMD/Readinova/apps/api/internal/billing"
)

// artefactRoutes adds audit artefact endpoints to the mux.
func (s *Server) artefactRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/assessments/{id}/artefacts", s.withAuth(s.handleCreateArtefact))
	mux.HandleFunc("GET /v1/assessments/{id}/artefacts", s.withAuth(s.handleListArtefacts))
}

// POST /v1/assessments/{id}/artefacts — sign and persist the latest scoring result.
func (s *Server) handleCreateArtefact(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")

	// Tier gate: audit artefacts require Growth or Enterprise.
	var tier string
	_ = s.db.QueryRow(r.Context(),
		`SELECT tier FROM subscriptions WHERE organisation_id = $1`, claims.OrgID,
	).Scan(&tier)
	if !billing.LimitsFor(billing.Tier(tier)).AuditArtefacts {
		writeError(w, http.StatusPaymentRequired, "audit artefacts require Growth tier or above")
		return
	}

	// Verify assessment belongs to org.
	var orgID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT organisation_id FROM assessments WHERE id = $1`, assessmentID,
	).Scan(&orgID); err != nil || orgID != claims.OrgID {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}

	// Load latest completed scoring run.
	var scoringRunID string
	var resultJSON []byte
	if err := s.db.QueryRow(r.Context(),
		`SELECT id, result_json FROM scoring_runs
		 WHERE assessment_id = $1 AND status = 'completed'
		 ORDER BY created_at DESC LIMIT 1`,
		assessmentID,
	).Scan(&scoringRunID, &resultJSON); err != nil {
		writeError(w, http.StatusConflict, "no completed scoring run found")
		return
	}

	var sr struct {
		CompositeLayerA  float64            `json:"composite_layer_a"`
		DimensionScores  map[string]float64 `json:"dimension_scores"`
		FrameworkVersion string             `json:"framework_version"`
		EngineVersion    string             `json:"engine_version"`
	}
	if err := json.Unmarshal(resultJSON, &sr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse scoring result")
		return
	}

	// Generate a fresh key pair for this artefact (each artefact is self-contained).
	kp, err := artefact.GenerateKeyPair()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "key generation failed")
		return
	}

	payload := artefact.Payload{
		AssessmentID:     assessmentID,
		OrganisationID:   claims.OrgID,
		ScoringRunID:     scoringRunID,
		CompositeScore:   sr.CompositeLayerA,
		DimensionScores:  sr.DimensionScores,
		FrameworkVersion: sr.FrameworkVersion,
		EngineVersion:    sr.EngineVersion,
	}

	signed, err := artefact.Sign(payload, kp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "signing failed")
		return
	}

	payloadBytes, _ := json.Marshal(signed.Payload)

	// Upsert the org's public key for reference.
	_, _ = s.db.Exec(r.Context(),
		`INSERT INTO signing_keys (organisation_id, public_key_b64) VALUES ($1,$2)
		 ON CONFLICT (organisation_id) DO UPDATE SET public_key_b64 = $2`,
		claims.OrgID, signed.PublicKeyB64,
	)

	var artefactID string
	if err := s.db.QueryRow(r.Context(),
		`INSERT INTO audit_artefacts
		 (organisation_id, assessment_id, scoring_run_id, payload_json, payload_hash, signature_b64, public_key_b64)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		claims.OrgID, assessmentID, scoringRunID,
		payloadBytes, signed.PayloadHash, signed.SignatureB64, signed.PublicKeyB64,
	).Scan(&artefactID); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           artefactID,
		"payload_hash": signed.PayloadHash,
		"signature":    signed.SignatureB64,
		"public_key":   signed.PublicKeyB64,
		"signed_at":    signed.Payload.SignedAt,
	})
}

// GET /v1/assessments/{id}/artefacts — list signed artefacts.
func (s *Server) handleListArtefacts(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")

	var orgID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT organisation_id FROM assessments WHERE id = $1`, assessmentID,
	).Scan(&orgID); err != nil || orgID != claims.OrgID {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}

	rows, err := s.db.Query(r.Context(),
		`SELECT id, payload_hash, signature_b64, public_key_b64, created_at
		 FROM audit_artefacts WHERE assessment_id = $1 ORDER BY created_at DESC`,
		assessmentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	type row struct {
		ID           string `json:"id"`
		PayloadHash  string `json:"payload_hash"`
		SignatureB64 string `json:"signature_b64"`
		PublicKeyB64 string `json:"public_key_b64"`
		CreatedAt    string `json:"created_at"`
	}
	var list []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ID, &x.PayloadHash, &x.SignatureB64, &x.PublicKeyB64, &x.CreatedAt); err != nil {
			continue
		}
		list = append(list, x)
	}
	if list == nil {
		list = []row{}
	}
	writeJSON(w, http.StatusOK, list)
}

