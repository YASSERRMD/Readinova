package httpapi

import (
	"encoding/json"
	"net/http"

	scoring "github.com/YASSERRMD/Readinova/libs/go-scoring/scoring"
)

func init() {
	// Scoring routes registered in server.go via scoringRoutes()
}

// scoringRoutes adds scoring endpoints to the mux.
func (s *Server) scoringRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/assessments/{id}/score", s.withAuth(s.handleTriggerScore))
	mux.HandleFunc("GET /v1/assessments/{id}/score", s.withAuth(s.handleGetScore))
}

// POST /v1/assessments/{id}/score
func (s *Server) handleTriggerScore(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")

	if claims.Role != "owner" && claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "only owner or admin may trigger scoring")
		return
	}

	// Verify assessment is ready_to_score.
	var status, frameworkID, orgID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT status, framework_id, organisation_id FROM assessments WHERE id = $1 AND organisation_id = $2`,
		assessmentID, claims.OrgID,
	).Scan(&status, &frameworkID, &orgID); err != nil {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}
	if status != "ready_to_score" {
		writeError(w, http.StatusConflict, "assessment must be ready_to_score to score")
		return
	}

	// Create a pending scoring_run.
	var runID string
	if err := s.db.QueryRow(r.Context(),
		`INSERT INTO scoring_runs (assessment_id, organisation_id, triggered_by, status, started_at)
		 VALUES ($1,$2,$3,'running',now()) RETURNING id`,
		assessmentID, orgID, claims.UserID,
	).Scan(&runID); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	// Build framework definition from DB.
	fw, err := s.loadFramework(r, frameworkID)
	if err != nil {
		s.failScoringRun(r, runID, err.Error())
		writeError(w, http.StatusInternalServerError, "failed to load framework")
		return
	}

	// Load responses.
	responses, err := s.loadResponses(r, assessmentID)
	if err != nil {
		s.failScoringRun(r, runID, err.Error())
		writeError(w, http.StatusInternalServerError, "failed to load responses")
		return
	}

	// Call the Rust scoring engine.
	result, err := scoring.Score(r.Context(), *fw, responses)
	if err != nil {
		s.failScoringRun(r, runID, err.Error())
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Persist result.
	resultJSON, _ := json.Marshal(result)
	if _, err := s.db.Exec(r.Context(),
		`UPDATE scoring_runs
		 SET status='completed', result_json=$1, engine_version=$2, completed_at=now()
		 WHERE id=$3`,
		resultJSON, result.EngineVersion, runID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	// Advance assessment to scored.
	if _, err := s.db.Exec(r.Context(),
		`UPDATE assessments SET status='scored', updated_at=now() WHERE id=$1`,
		assessmentID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	_, _ = s.db.Exec(r.Context(),
		`INSERT INTO audit_log (organisation_id, user_id, action, target_type, target_id)
		 VALUES ($1,$2,'scoring_completed','scoring_run',$3)`,
		orgID, claims.UserID, runID)

	writeJSON(w, http.StatusOK, map[string]any{
		"run_id":             runID,
		"composite_layer_a":  result.CompositeLayerA,
		"dimension_scores":   result.DimensionScores,
		"binding_constraint": result.BindingConstraintDimension,
	})
}

// GET /v1/assessments/{id}/score
func (s *Server) handleGetScore(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")

	// Verify org access.
	var orgID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT organisation_id FROM assessments WHERE id = $1`, assessmentID,
	).Scan(&orgID); err != nil || orgID != claims.OrgID {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}

	var resultJSON []byte
	var engineVersion string
	if err := s.db.QueryRow(r.Context(),
		`SELECT result_json, engine_version FROM scoring_runs
		 WHERE assessment_id = $1 AND status = 'completed'
		 ORDER BY created_at DESC LIMIT 1`,
		assessmentID,
	).Scan(&resultJSON, &engineVersion); err != nil {
		writeError(w, http.StatusNotFound, "no completed scoring run found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resultJSON)
}

// loadFramework queries the full framework hierarchy for scoring.
func (s *Server) loadFramework(r *http.Request, frameworkID string) (*scoring.Framework, error) {
	var fw scoring.Framework
	if err := s.db.QueryRow(r.Context(),
		`SELECT slug, version_major::text || '.' || version_minor::text FROM frameworks WHERE id = $1`,
		frameworkID,
	).Scan(&fw.Slug, &fw.Version); err != nil {
		return nil, err
	}

	// Load dimensions.
	dimRows, err := s.db.Query(r.Context(),
		`SELECT id, slug, default_weight FROM dimensions WHERE framework_id = $1 ORDER BY display_order`,
		frameworkID,
	)
	if err != nil {
		return nil, err
	}
	defer dimRows.Close()

	type dimRecord struct {
		id  string
		dim scoring.DimensionDef
	}
	var dims []dimRecord
	for dimRows.Next() {
		var rec dimRecord
		if err := dimRows.Scan(&rec.id, &rec.dim.Slug, &rec.dim.Weight); err != nil {
			continue
		}
		dims = append(dims, rec)
	}
	dimRows.Close()

	for i := range dims {
		sdRows, err := s.db.Query(r.Context(),
			`SELECT id, slug, weight_within_dimension FROM sub_dimensions WHERE dimension_id = $1 ORDER BY display_order`,
			dims[i].id,
		)
		if err != nil {
			return nil, err
		}

		type sdRecord struct {
			id string
			sd scoring.SubDimensionDef
		}
		var sds []sdRecord
		for sdRows.Next() {
			var rec sdRecord
			if err := sdRows.Scan(&rec.id, &rec.sd.Slug, &rec.sd.Weight); err != nil {
				continue
			}
			sds = append(sds, rec)
		}
		sdRows.Close()

		for j := range sds {
			indRows, err := s.db.Query(r.Context(),
				`SELECT id, slug FROM indicators WHERE sub_dimension_id = $1 ORDER BY display_order`,
				sds[j].id,
			)
			if err != nil {
				return nil, err
			}

			type indRecord struct {
				id  string
				ind scoring.IndicatorDef
			}
			var inds []indRecord
			for indRows.Next() {
				var rec indRecord
				if err := indRows.Scan(&rec.id, &rec.ind.Slug); err != nil {
					continue
				}
				inds = append(inds, rec)
			}
			indRows.Close()

			for k := range inds {
				qRows, err := s.db.Query(r.Context(),
					`SELECT slug FROM questions WHERE indicator_id = $1 ORDER BY display_order`,
					inds[k].id,
				)
				if err != nil {
					return nil, err
				}
				for qRows.Next() {
					var q scoring.QuestionDef
					if err := qRows.Scan(&q.Slug); err != nil {
						continue
					}
					inds[k].ind.Questions = append(inds[k].ind.Questions, q)
				}
				qRows.Close()
				sds[j].sd.Indicators = append(sds[j].sd.Indicators, inds[k].ind)
			}
			dims[i].dim.SubDimensions = append(dims[i].dim.SubDimensions, sds[j].sd)
		}
		fw.Dimensions = append(fw.Dimensions, dims[i].dim)
	}
	return &fw, nil
}

// loadResponses queries all responses for an assessment.
func (s *Server) loadResponses(r *http.Request, assessmentID string) ([]scoring.Response, error) {
	rows, err := s.db.Query(r.Context(),
		`SELECT q.slug, rsp.level
		 FROM responses rsp
		 JOIN questions q ON q.id = rsp.question_id
		 WHERE rsp.assessment_id = $1`,
		assessmentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var responses []scoring.Response
	for rows.Next() {
		var resp scoring.Response
		var level int
		if err := rows.Scan(&resp.QuestionSlug, &level); err != nil {
			continue
		}
		resp.Level = uint8(level) //nolint:gosec
		responses = append(responses, resp)
	}
	return responses, nil
}

// failScoringRun marks a scoring run as failed with an error message.
func (s *Server) failScoringRun(r *http.Request, runID, msg string) {
	_, _ = s.db.Exec(r.Context(),
		`UPDATE scoring_runs SET status='failed', error_message=$1, completed_at=now() WHERE id=$2`,
		msg, runID,
	)
}
