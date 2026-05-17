package httpapi

import (
	"fmt"
	"net/http"

	"github.com/YASSERRMD/Readinova/apps/api/internal/billing"
)

// assessmentRoutes adds assessment endpoints to the mux.
func (s *Server) assessmentRoutes(mux *http.ServeMux) {
	// Frameworks (public within org — no tier gate needed).
	mux.HandleFunc("GET /v1/frameworks", s.withAuth(s.handleListFrameworks))

	mux.HandleFunc("POST /v1/assessments", s.withAuth(s.handleCreateAssessment))
	mux.HandleFunc("GET /v1/assessments", s.withAuth(s.handleListAssessments))
	mux.HandleFunc("GET /v1/assessments/{id}", s.withAuth(s.handleGetAssessment))
	mux.HandleFunc("POST /v1/assessments/{id}/assignments", s.withAuth(s.handleSetAssignments))
	mux.HandleFunc("POST /v1/assessments/{id}/start", s.withAuth(s.handleStartAssessment))
	mux.HandleFunc("GET /v1/assessments/{id}/questions", s.withAuth(s.handleListQuestions))
	mux.HandleFunc("POST /v1/assessments/{id}/submit", s.withAuth(s.handleSubmitAssessment))
}

// GET /v1/frameworks — list all available assessment frameworks.
func (s *Server) handleListFrameworks(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(),
		`SELECT id, slug, name, description, version_major, version_minor
		 FROM frameworks ORDER BY name`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	type fw struct {
		ID          string `json:"id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Version     string `json:"version"`
	}
	var list []fw
	for rows.Next() {
		var f fw
		var vMaj, vMin int
		if err := rows.Scan(&f.ID, &f.Slug, &f.Name, &f.Description, &vMaj, &vMin); err != nil {
			continue
		}
		f.Version = fmt.Sprintf("%d.%d", vMaj, vMin)
		list = append(list, f)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if list == nil {
		list = []fw{}
	}
	writeJSON(w, http.StatusOK, list)
}

// POST /v1/assessments
func (s *Server) handleCreateAssessment(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	var req struct {
		FrameworkID string `json:"framework_id"`
		Title       string `json:"title"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.FrameworkID == "" || req.Title == "" {
		writeError(w, http.StatusUnprocessableEntity, "framework_id and title are required")
		return
	}

	// Enforce tier assessment limit atomically: the conditional INSERT ensures
	// the count check and the insert happen in the same statement, preventing
	// the TOCTOU race that exists when counting then inserting separately.
	limits := billing.LimitsFor(s.tierFor(r.Context(), claims.OrgID))
	maxAssessments := limits.MaxAssessments

	var id string
	var insertErr error
	if maxAssessments > 0 {
		insertErr = s.db.QueryRow(r.Context(),
			`INSERT INTO assessments (organisation_id, framework_id, title)
			 SELECT $1,$2,$3
			 WHERE (SELECT COUNT(*) FROM assessments WHERE organisation_id = $1) < $4
			 RETURNING id`,
			claims.OrgID, req.FrameworkID, req.Title, maxAssessments,
		).Scan(&id)
	} else {
		insertErr = s.db.QueryRow(r.Context(),
			`INSERT INTO assessments (organisation_id, framework_id, title)
			 VALUES ($1,$2,$3) RETURNING id`,
			claims.OrgID, req.FrameworkID, req.Title,
		).Scan(&id)
	}
	if insertErr != nil {
		// Distinguish limit hit (no rows returned) from actual DB error.
		if id == "" && maxAssessments > 0 {
			writeError(w, http.StatusPaymentRequired, "assessment limit reached for your tier")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid framework_id or db error")
		return
	}

	_, _ = s.db.Exec(r.Context(),
		`INSERT INTO audit_log (organisation_id, user_id, action, target_type, target_id)
		 VALUES ($1,$2,'assessment_created','assessment',$3)`,
		claims.OrgID, claims.UserID, id)

	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "draft"})
}

// GET /v1/assessments
func (s *Server) handleListAssessments(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)

	rows, err := s.db.Query(r.Context(),
		`SELECT id, framework_id, title, status, created_at
		 FROM assessments WHERE organisation_id = $1 ORDER BY created_at DESC`,
		claims.OrgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	type row struct {
		ID          string `json:"id"`
		FrameworkID string `json:"framework_id"`
		Title       string `json:"title"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
	}
	var list []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ID, &x.FrameworkID, &x.Title, &x.Status, &x.CreatedAt); err != nil {
			continue
		}
		list = append(list, x)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if list == nil {
		list = []row{}
	}
	writeJSON(w, http.StatusOK, list)
}

// GET /v1/assessments/{id}
func (s *Server) handleGetAssessment(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	id := r.PathValue("id")

	type progress struct {
		Role     string `json:"role"`
		Total    int    `json:"total"`
		Answered int    `json:"answered"`
		Pct      int    `json:"pct"`
	}
	type result struct {
		ID          string     `json:"id"`
		Title       string     `json:"title"`
		Status      string     `json:"status"`
		FrameworkID string     `json:"framework_id"`
		Progress    []progress `json:"progress"`
	}

	var res result
	if err := s.db.QueryRow(r.Context(),
		`SELECT id, title, status, framework_id FROM assessments
		 WHERE id = $1 AND organisation_id = $2`,
		id, claims.OrgID,
	).Scan(&res.ID, &res.Title, &res.Status, &res.FrameworkID); err != nil {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}

	rows, err := s.db.Query(r.Context(), `
		SELECT qa.assigned_role,
		       COUNT(*) AS total,
		       COUNT(rsp.id) AS answered
		FROM question_assignments qa
		LEFT JOIN responses rsp
		    ON rsp.assessment_id = qa.assessment_id AND rsp.question_id = qa.question_id
		WHERE qa.assessment_id = $1
		GROUP BY qa.assigned_role
	`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p progress
			if err := rows.Scan(&p.Role, &p.Total, &p.Answered); err != nil {
				continue
			}
			if p.Total > 0 {
				p.Pct = (p.Answered * 100) / p.Total
			}
			res.Progress = append(res.Progress, p)
		}
	}
	if res.Progress == nil {
		res.Progress = []progress{}
	}
	writeJSON(w, http.StatusOK, res)
}

// POST /v1/assessments/{id}/assignments — set role→user map and partition questions.
func (s *Server) handleSetAssignments(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")

	// Verify ownership.
	var orgID, status string
	if err := s.db.QueryRow(r.Context(),
		`SELECT organisation_id, status FROM assessments WHERE id = $1`,
		assessmentID,
	).Scan(&orgID, &status); err != nil || orgID != claims.OrgID {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}
	if status != "draft" {
		writeError(w, http.StatusConflict, "assignments can only be set on draft assessments")
		return
	}

	// role → user_id map from request.
	var req struct {
		Assignments map[string]string `json:"assignments"` // role → user_id
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Load all questions for this framework with their target_role.
	type qrow struct {
		QuestionID string
		TargetRole string
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT q.id, q.target_role
		FROM questions q
		JOIN indicators ind ON ind.id = q.indicator_id
		JOIN sub_dimensions sd ON sd.id = ind.sub_dimension_id
		JOIN dimensions d ON d.id = sd.dimension_id
		JOIN assessments a ON a.framework_id = d.framework_id
		WHERE a.id = $1
	`, assessmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	var questions []qrow
	missingRoles := map[string]bool{}
	for rows.Next() {
		var q qrow
		if err := rows.Scan(&q.QuestionID, &q.TargetRole); err != nil {
			continue
		}
		if q.TargetRole != "any" {
			if _, ok := req.Assignments[q.TargetRole]; !ok {
				missingRoles[q.TargetRole] = true
			}
		}
		questions = append(questions, q)
	}

	if len(questions) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "no questions found for this assessment's framework")
		return
	}

	if len(missingRoles) > 0 {
		roles := make([]string, 0, len(missingRoles))
		for r := range missingRoles {
			roles = append(roles, r)
		}
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":         "missing role assignments",
			"missing_roles": roles,
		})
		return
	}

	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Delete existing assignments.
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM question_assignments WHERE assessment_id = $1`, assessmentID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM assessment_assignments WHERE assessment_id = $1`, assessmentID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	// Insert per-role assessment_assignments.
	for role, userID := range req.Assignments {
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO assessment_assignments (assessment_id, user_id, role) VALUES ($1,$2,$3)
			 ON CONFLICT DO NOTHING`,
			assessmentID, userID, role,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
	}

	// Insert question_assignments.
	ownerUserID := claims.UserID
	for _, q := range questions {
		assignedRole := q.TargetRole
		var assignedUser *string
		if q.TargetRole == "any" {
			assignedRole = "any"
			assignedUser = &ownerUserID
		} else if uid, ok := req.Assignments[q.TargetRole]; ok {
			assignedUser = &uid
		}

		if _, err := tx.Exec(r.Context(),
			`INSERT INTO question_assignments (assessment_id, question_id, assigned_role, assigned_user_id)
			 VALUES ($1,$2,$3,$4)`,
			assessmentID, q.QuestionID, assignedRole, assignedUser,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"partitioned": len(questions),
		"roles":       len(req.Assignments),
	})
}

// POST /v1/assessments/{id}/start
func (s *Server) handleStartAssessment(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	id := r.PathValue("id")

	tag, err := s.db.Exec(r.Context(),
		`UPDATE assessments
		 SET status = 'in_progress', started_at = now(), updated_at = now()
		 WHERE id = $1 AND organisation_id = $2 AND status = 'draft'`,
		id, claims.OrgID)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusConflict, "assessment not found or not in draft status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "in_progress"})
}

// GET /v1/assessments/{id}/questions?role=cio
func (s *Server) handleListQuestions(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	id := r.PathValue("id")
	role := r.URL.Query().Get("role")

	// Verify assessment belongs to org.
	var orgID string
	if err := s.db.QueryRow(r.Context(),
		`SELECT organisation_id FROM assessments WHERE id = $1`, id,
	).Scan(&orgID); err != nil || orgID != claims.OrgID {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}

	query := `
		SELECT q.id, q.slug, q.prompt, q.target_role, qa.assigned_role, qa.assigned_user_id
		FROM question_assignments qa
		JOIN questions q ON q.id = qa.question_id
		WHERE qa.assessment_id = $1`
	args := []any{id}
	if role != "" {
		query += " AND qa.assigned_role = $2"
		args = append(args, role)
	}
	query += " ORDER BY q.display_order"

	rows, err := s.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	type rubricLevel struct {
		Level       int    `json:"level"`
		Label       string `json:"label"`
		Description string `json:"description"`
	}
	type qrow struct {
		ID             string        `json:"id"`
		Slug           string        `json:"slug"`
		Prompt         string        `json:"prompt"`
		TargetRole     string        `json:"target_role"`
		AssignedRole   string        `json:"assigned_role"`
		AssignedUserID *string       `json:"assigned_user_id,omitempty"`
		RubricLevels   []rubricLevel `json:"rubric_levels"`
	}

	// Index questions by ID for O(1) rubric attachment below.
	var list []qrow
	indexByID := map[string]int{}
	for rows.Next() {
		var q qrow
		if err := rows.Scan(&q.ID, &q.Slug, &q.Prompt, &q.TargetRole, &q.AssignedRole, &q.AssignedUserID); err != nil {
			continue
		}
		q.RubricLevels = []rubricLevel{}
		indexByID[q.ID] = len(list)
		list = append(list, q)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	rows.Close()

	// Fetch ALL rubric levels for this assessment in a single query (no N+1).
	if len(list) > 0 {
		rlRows, err := s.db.Query(r.Context(), `
			SELECT rl.question_id, rl.level, rl.label, rl.description
			FROM rubric_levels rl
			JOIN question_assignments qa ON qa.question_id = rl.question_id
			WHERE qa.assessment_id = $1
			ORDER BY rl.question_id, rl.level
		`, id)
		if err == nil {
			defer rlRows.Close()
			for rlRows.Next() {
				var qID string
				var rl rubricLevel
				if err := rlRows.Scan(&qID, &rl.Level, &rl.Label, &rl.Description); err != nil {
					continue
				}
				if idx, ok := indexByID[qID]; ok {
					list[idx].RubricLevels = append(list[idx].RubricLevels, rl)
				}
			}
		}
	}

	if list == nil {
		list = []qrow{}
	}
	writeJSON(w, http.StatusOK, list)
}

// POST /v1/assessments/{id}/submit
func (s *Server) handleSubmitAssessment(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	id := r.PathValue("id")

	// Verify status = in_progress.
	var status string
	if err := s.db.QueryRow(r.Context(),
		`SELECT status FROM assessments WHERE id = $1 AND organisation_id = $2`,
		id, claims.OrgID,
	).Scan(&status); err != nil {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}
	if status != "in_progress" {
		writeError(w, http.StatusConflict, "assessment must be in_progress to submit")
		return
	}

	// Check all questions answered.
	var total, answered int
	if err := s.db.QueryRow(r.Context(), `
		SELECT COUNT(*) AS total,
		       COUNT(rsp.id) AS answered
		FROM question_assignments qa
		LEFT JOIN responses rsp
		    ON rsp.assessment_id = qa.assessment_id AND rsp.question_id = qa.question_id
		WHERE qa.assessment_id = $1
	`, id).Scan(&total, &answered); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	if answered < total {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":    "not all questions answered",
			"total":    total,
			"answered": answered,
			"missing":  total - answered,
		})
		return
	}

	if _, err := s.db.Exec(r.Context(),
		`UPDATE assessments
		 SET status = 'ready_to_score', completed_at = now(), updated_at = now()
		 WHERE id = $1`,
		id,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	_, _ = s.db.Exec(r.Context(),
		`INSERT INTO audit_log (organisation_id, user_id, action, target_type, target_id)
		 VALUES ($1,$2,'assessment_submitted','assessment',$3)`,
		claims.OrgID, claims.UserID, id)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready_to_score"})
}
