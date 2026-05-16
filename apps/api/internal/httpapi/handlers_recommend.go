package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/YASSERRMD/Readinova/apps/api/internal/billing"
	"github.com/YASSERRMD/Readinova/apps/api/internal/recommend"
)

// recommendRoutes adds recommendation endpoints to the mux.
func (s *Server) recommendRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/assessments/{id}/recommendations", s.withAuth(s.handleGetRecommendations))
}

// GET /v1/assessments/{id}/recommendations
func (s *Server) handleGetRecommendations(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")

	// Tier gate: recommendation engine requires Starter or above.
	if !billing.LimitsFor(s.tierFor(r.Context(), claims.OrgID)).RecommendationEngine {
		writeError(w, http.StatusPaymentRequired, "recommendation engine requires Starter tier or above")
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

	// Load latest scoring result for dimension scores.
	var resultJSON []byte
	if err := s.db.QueryRow(r.Context(),
		`SELECT result_json FROM scoring_runs
		 WHERE assessment_id = $1 AND status = 'completed'
		 ORDER BY created_at DESC LIMIT 1`,
		assessmentID,
	).Scan(&resultJSON); err != nil {
		writeError(w, http.StatusConflict, "no completed scoring run found")
		return
	}

	var sr struct {
		DimensionScores map[string]float64 `json:"dimension_scores"`
	}
	if err := json.Unmarshal(resultJSON, &sr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse scoring result")
		return
	}

	recs, err := recommend.Generate(sr.DimensionScores)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "recommendation engine error")
		return
	}

	// Group by wave for the response.
	waves := map[int][]recommend.Recommendation{}
	for _, rec := range recs {
		waves[rec.Wave] = append(waves[rec.Wave], rec)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":           len(recs),
		"recommendations": recs,
		"waves": map[string]any{
			"wave_1": waves[1],
			"wave_2": waves[2],
			"wave_3": waves[3],
		},
	})
}
