package httpapi

import (
	"net/http"
)

func init() {
	// Response routes registered in server.go via responseRoutes()
}

// responseRoutes adds response intake endpoints to the mux.
func (s *Server) responseRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PUT /v1/assessments/{id}/responses/{slug}", s.withAuth(s.handleUpsertResponse))
	mux.HandleFunc("GET /v1/assessments/{id}/responses", s.withAuth(s.handleListResponses))
	mux.HandleFunc("DELETE /v1/assessments/{id}/responses/{slug}", s.withAuth(s.handleDeleteResponse))
}

// PUT /v1/assessments/{id}/responses/{slug}
func (s *Server) handleUpsertResponse(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")
	questionSlug := r.PathValue("slug")

	// Verify assessment belongs to org and is still editable.
	var status string
	if err := s.db.QueryRow(r.Context(),
		`SELECT status FROM assessments WHERE id = $1 AND organisation_id = $2`,
		assessmentID, claims.OrgID,
	).Scan(&status); err != nil {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}
	if status != "draft" && status != "in_progress" {
		writeError(w, http.StatusPreconditionFailed, "assessment is no longer accepting responses")
		return
	}

	var req struct {
		Level    int     `json:"level"`
		FreeText *string `json:"free_text"`
		Evidence []struct {
			Kind string `json:"kind"`
			Ref  string `json:"ref"`
		} `json:"evidence"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Level < 0 || req.Level > 4 {
		writeError(w, http.StatusUnprocessableEntity, "level must be 0-4")
		return
	}

	// Look up the question.
	var questionID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT id FROM questions WHERE slug = $1`, questionSlug,
	).Scan(&questionID); err != nil {
		writeError(w, http.StatusNotFound, "question not found")
		return
	}

	// Validate rubric level.
	var rubricExists bool
	if err := s.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM rubric_levels WHERE question_id = $1 AND level = $2)`,
		questionID, req.Level,
	).Scan(&rubricExists); err != nil || !rubricExists {
		writeError(w, http.StatusUnprocessableEntity, "invalid rubric level for this question")
		return
	}

	// Verify the calling user is assigned to this question (unless admin/owner).
	if claims.Role != "owner" && claims.Role != "admin" {
		var assignedUser *string
		if err := s.db.QueryRow(r.Context(),
			`SELECT assigned_user_id FROM question_assignments
			 WHERE assessment_id = $1 AND question_id = $2`,
			assessmentID, questionID,
		).Scan(&assignedUser); err != nil {
			writeError(w, http.StatusForbidden, "question not assigned to this user")
			return
		}
		if assignedUser == nil || *assignedUser != claims.UserID {
			writeError(w, http.StatusForbidden, "question not assigned to this user")
			return
		}
	}

	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	var respID string
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO responses (assessment_id, question_id, level, free_text, created_by)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (assessment_id, question_id)
		 DO UPDATE SET level=$3, free_text=$4, updated_at=now()
		 RETURNING id`,
		assessmentID, questionID, req.Level, req.FreeText, claims.UserID,
	).Scan(&respID); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	// Replace evidence.
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM response_evidence WHERE response_id = $1`, respID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	for _, ev := range req.Evidence {
		if ev.Kind == "" || ev.Ref == "" {
			continue
		}
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO response_evidence (response_id, kind, ref) VALUES ($1,$2,$3)`,
			respID, ev.Kind, ev.Ref,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
	}

	if _, err := tx.Exec(r.Context(),
		`INSERT INTO audit_log (organisation_id, user_id, action, target_type, target_id)
		 VALUES ($1,$2,'response_upserted','response',$3)`,
		claims.OrgID, claims.UserID, respID,
	); err != nil {
		// Non-fatal: audit failure should not block the response.
		_ = err
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":    respID,
		"level": req.Level,
	})
}

// GET /v1/assessments/{id}/responses?role=cio
func (s *Server) handleListResponses(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")
	role := r.URL.Query().Get("role")

	// Verify access.
	var orgID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT organisation_id FROM assessments WHERE id = $1`, assessmentID,
	).Scan(&orgID); err != nil || orgID != claims.OrgID {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}

	query := `
		SELECT q.slug, rsp.level, rsp.free_text, rsp.updated_at, qa.assigned_role
		FROM responses rsp
		JOIN questions q ON q.id = rsp.question_id
		JOIN question_assignments qa
		    ON qa.assessment_id = rsp.assessment_id AND qa.question_id = rsp.question_id
		WHERE rsp.assessment_id = $1`
	args := []any{assessmentID}
	if role != "" {
		query += " AND qa.assigned_role = $2"
		args = append(args, role)
	}
	query += " ORDER BY q.slug"

	rows, err := s.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	type row struct {
		QuestionSlug string  `json:"question_slug"`
		Level        int     `json:"level"`
		FreeText     *string `json:"free_text,omitempty"`
		UpdatedAt    string  `json:"updated_at"`
		AssignedRole string  `json:"assigned_role"`
	}
	var list []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.QuestionSlug, &x.Level, &x.FreeText, &x.UpdatedAt, &x.AssignedRole); err != nil {
			continue
		}
		list = append(list, x)
	}
	if list == nil {
		list = []row{}
	}
	writeJSON(w, http.StatusOK, list)
}

// DELETE /v1/assessments/{id}/responses/{slug}
func (s *Server) handleDeleteResponse(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")
	questionSlug := r.PathValue("slug")

	// Check assessment is still editable.
	var status string
	if err := s.db.QueryRow(r.Context(),
		`SELECT status FROM assessments WHERE id = $1 AND organisation_id = $2`,
		assessmentID, claims.OrgID,
	).Scan(&status); err != nil {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}
	if status != "draft" && status != "in_progress" {
		writeError(w, http.StatusPreconditionFailed, "assessment is no longer accepting changes")
		return
	}

	var questionID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT id FROM questions WHERE slug = $1`, questionSlug,
	).Scan(&questionID); err != nil {
		writeError(w, http.StatusNotFound, "question not found")
		return
	}

	tag, err := s.db.Exec(r.Context(),
		`DELETE FROM responses WHERE assessment_id = $1 AND question_id = $2`,
		assessmentID, questionID,
	)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "response not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
