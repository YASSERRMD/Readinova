package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/YASSERRMD/Readinova/apps/api/internal/perception"
)

// perceptionRoutes adds perception gap endpoints to the mux.
func (s *Server) perceptionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/assessments/{id}/perception-gap", s.withAuth(s.handleRunPerceptionGap))
	mux.HandleFunc("GET /v1/assessments/{id}/perception-gap", s.withAuth(s.handleGetPerceptionGap))
}

// POST /v1/assessments/{id}/perception-gap
// Computes Layer B from persisted evidence signals and stores the gap result.
func (s *Server) handleRunPerceptionGap(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")

	// Verify assessment belongs to org.
	var orgID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT organisation_id FROM assessments WHERE id = $1`,
		assessmentID,
	).Scan(&orgID); err != nil || orgID != claims.OrgID {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}

	// Load latest Layer A composite + dimension scores from scoring_runs.
	var resultJSON []byte
	if err := s.db.QueryRow(r.Context(),
		`SELECT result_json FROM scoring_runs
		 WHERE assessment_id = $1 AND status = 'completed'
		 ORDER BY created_at DESC LIMIT 1`,
		assessmentID,
	).Scan(&resultJSON); err != nil {
		writeError(w, http.StatusConflict, "no completed scoring run found; run scoring first")
		return
	}

	var sr struct {
		CompositeLayerA float64            `json:"composite_layer_a"`
		DimensionScores map[string]float64 `json:"dimension_scores"`
	}
	if err := json.Unmarshal(resultJSON, &sr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse scoring result")
		return
	}

	// Load evidence signals for this org.
	rows, err := s.db.Query(r.Context(),
		`SELECT connector_type, dimension_slug, signal_key, signal_value
		 FROM evidence_signals
		 WHERE organisation_id = $1
		 ORDER BY collected_at DESC`,
		claims.OrgID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	// Group signals by dimension.
	dimMap := map[string]*perception.DimensionEvidence{}
	for rows.Next() {
		var connType, dimSlug, sigKey string
		var sigVal json.RawMessage
		if err := rows.Scan(&connType, &dimSlug, &sigKey, &sigVal); err != nil {
			continue
		}
		if _, ok := dimMap[dimSlug]; !ok {
			dimMap[dimSlug] = &perception.DimensionEvidence{DimensionSlug: dimSlug}
		}
		dimMap[dimSlug].Signals = append(dimMap[dimSlug].Signals, perception.EvidenceSignal{
			ConnectorType: connType,
			SignalKey:     sigKey,
			SignalValue:   sigVal,
		})
	}

	evidence := make([]perception.DimensionEvidence, 0, len(dimMap))
	for _, de := range dimMap {
		evidence = append(evidence, *de)
	}

	layerB := perception.ComputeLayerB(evidence)
	gapResult := perception.ComputeGap(sr.CompositeLayerA, sr.DimensionScores, layerB)

	resultBytes, _ := json.Marshal(gapResult)

	var runID string
	if err := s.db.QueryRow(r.Context(),
		`INSERT INTO perception_gap_runs
		 (assessment_id, organisation_id, layer_a_score, layer_b_score, gap_score, master_composite, result_json)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		assessmentID, claims.OrgID,
		gapResult.LayerAScore, gapResult.LayerBScore, gapResult.GapScore, gapResult.MasterComposite,
		resultBytes,
	).Scan(&runID); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"run_id":           runID,
		"layer_a_score":    gapResult.LayerAScore,
		"layer_b_score":    gapResult.LayerBScore,
		"gap_score":        gapResult.GapScore,
		"master_composite": gapResult.MasterComposite,
	})
}

// GET /v1/assessments/{id}/perception-gap — returns the latest gap run.
func (s *Server) handleGetPerceptionGap(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")

	var orgID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT organisation_id FROM assessments WHERE id = $1`, assessmentID,
	).Scan(&orgID); err != nil || orgID != claims.OrgID {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}

	var resultJSON []byte
	var layerA, layerB, gap, master float64
	if err := s.db.QueryRow(r.Context(),
		`SELECT layer_a_score, layer_b_score, gap_score, master_composite, result_json
		 FROM perception_gap_runs
		 WHERE assessment_id = $1
		 ORDER BY created_at DESC LIMIT 1`,
		assessmentID,
	).Scan(&layerA, &layerB, &gap, &master, &resultJSON); err != nil {
		writeError(w, http.StatusNotFound, "no perception gap run found")
		return
	}

	var detail any
	_ = json.Unmarshal(resultJSON, &detail)

	writeJSON(w, http.StatusOK, map[string]any{
		"layer_a_score":    layerA,
		"layer_b_score":    layerB,
		"gap_score":        gap,
		"master_composite": master,
		"detail":           detail,
	})
}
