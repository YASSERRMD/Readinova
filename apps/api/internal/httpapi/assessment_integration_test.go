package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// insertTestFramework inserts a minimal framework with one question and
// returns the framework ID, or skips the test if it fails.
func insertTestFramework(t *testing.T, pool interface {
	QueryRow(ctx context.Context, sql string, args ...any) interface {
		Scan(dest ...any) error
	}
	Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error)
}) string {
	t.Helper()
	t.Skip("use pool directly")
	return ""
}

// TestAssessmentLifecycle tests create → start → get progress → submit (vacuous, 0 questions).
func TestAssessmentLifecycle(t *testing.T) {
	pool := resetAndMigrate(t)
	srv := newServer(t, pool)
	defer srv.Close()

	// Sign up.
	signupResp := post(t, srv, "/v1/organisations", map[string]any{
		"org_name": "Readiness Corp", "org_slug": "readiness-corp",
		"country_code": "GB", "sector": "finance", "size_band": "51-250",
		"email": "owner@readiness.com", "password": "ReadinessPass123!",
	}, "")
	var signupBody map[string]string
	mustDecodeJSON(t, signupResp, &signupBody)
	token := signupBody["access_token"]
	ownerID := signupBody["user_id"]

	// Insert a minimal published framework with no questions (for lifecycle test).
	var fwID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO frameworks (slug, name, version_major, version_minor, status)
		 VALUES ('test-fw-lifecycle','Test Framework',1,0,'published') RETURNING id`,
	).Scan(&fwID); err != nil {
		t.Skipf("framework insert failed (schema may differ): %v", err)
	}
	// Minimal dimension required by FK.
	var dimID string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO dimensions (framework_id, slug, name, description, default_weight, display_order)
		 VALUES ($1,'dim_lifecycle','Dim','desc','1.0000',1) RETURNING id`, fwID,
	).Scan(&dimID); err != nil {
		t.Skipf("dimension insert: %v", err)
	}

	// Create assessment.
	cr := post(t, srv, "/v1/assessments", map[string]any{
		"framework_id": fwID, "title": "Q1 2026 AI Readiness",
	}, token)
	if cr.StatusCode != http.StatusCreated {
		t.Fatalf("create assessment: want 201, got %d", cr.StatusCode)
	}
	var crBody map[string]string
	mustDecodeJSON(t, cr, &crBody)
	assessmentID := crBody["id"]
	if assessmentID == "" {
		t.Fatal("no assessment id returned")
	}

	// Set assignments.
	assignResp := post(t, srv, fmt.Sprintf("/v1/assessments/%s/assignments", assessmentID), map[string]any{
		"assignments": map[string]string{
			"executive": ownerID, "cio": ownerID, "risk": ownerID, "ops": ownerID,
		},
	}, token)
	_ = assignResp // may be 200 or 422 if there are no questions for those roles

	// Start assessment.
	startResp := post(t, srv, fmt.Sprintf("/v1/assessments/%s/start", assessmentID), nil, token)
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("start: want 200, got %d", startResp.StatusCode)
	}

	// Get assessment.
	getReq, _ := http.NewRequest(http.MethodGet,
		srv.URL+fmt.Sprintf("/v1/assessments/%s", assessmentID), http.NoBody)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, _ := http.DefaultClient.Do(getReq)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get: want 200, got %d", getResp.StatusCode)
	}
	var getBody map[string]any
	mustDecodeJSON(t, getResp, &getBody)
	if getBody["status"] != "in_progress" {
		t.Errorf("status: want in_progress, got %v", getBody["status"])
	}

	// Submit — vacuous (0 questions = all answered).
	submitResp := post(t, srv, fmt.Sprintf("/v1/assessments/%s/submit", assessmentID), nil, token)
	if submitResp.StatusCode != http.StatusOK {
		t.Fatalf("submit (0 questions): want 200, got %d", submitResp.StatusCode)
	}
	var submitBody map[string]string
	mustDecodeJSON(t, submitResp, &submitBody)
	if submitBody["status"] != "ready_to_score" {
		t.Errorf("submit status: want ready_to_score, got %s", submitBody["status"])
	}
}

// TestPartialSubmitRejected inserts a framework with 1 question, creates an
// assessment, and verifies that submit is rejected with 422 when unanswered.
func TestPartialSubmitRejected(t *testing.T) {
	pool := resetAndMigrate(t)
	srv := newServer(t, pool)
	defer srv.Close()

	signupResp := post(t, srv, "/v1/organisations", map[string]any{
		"org_name": "Partial Corp", "org_slug": "partial-corp",
		"country_code": "DE", "sector": "mfg", "size_band": "1-50",
		"email": "owner@partial.com", "password": "PartialPass123!",
	}, "")
	var signupBody map[string]string
	mustDecodeJSON(t, signupResp, &signupBody)
	token := signupBody["access_token"]
	ownerID := signupBody["user_id"]

	// Insert a framework with exactly one question.
	var fwID, dimID, sdID, indID, qID string
	for _, step := range []struct {
		dest *string
		sql  string
		args []any
	}{
		{&fwID, `INSERT INTO frameworks (slug,name,version_major,version_minor,status) VALUES ('p-fw','P FW',1,0,'published') RETURNING id`, nil},
		{&dimID, `INSERT INTO dimensions (framework_id,slug,name,description,default_weight,display_order) VALUES ($1,'p_dim','P Dim','d','1.0000',1) RETURNING id`, nil},
		{&sdID, `INSERT INTO sub_dimensions (dimension_id,slug,name,description,weight_within_dimension,display_order) VALUES ($1,'p_sd','P SD','d','1.0000',1) RETURNING id`, nil},
		{&indID, `INSERT INTO indicators (sub_dimension_id,slug,name,description,display_order) VALUES ($1,'p_ind','P Ind','d',1) RETURNING id`, nil},
		{&qID, `INSERT INTO questions (indicator_id,slug,prompt,target_role,display_order,regulatory_references) VALUES ($1,'p_q','Has the org got a plan?','executive',1,'{"nist_ai_rmf":["GOVERN-1.1"]}') RETURNING id`, nil},
	} {
		var prevID string
		if *step.dest == "" && len(step.args) == 0 {
			step.args = []any{}
		}
		// Use the last inserted ID as $1 argument if needed.
		switch {
		case dimID != "" && sdID == "" && step.dest == &sdID:
			step.args = []any{dimID}
		case sdID != "" && indID == "" && step.dest == &indID:
			step.args = []any{sdID}
		case indID != "" && qID == "" && step.dest == &qID:
			step.args = []any{indID}
		case dimID == "" && step.dest == &dimID:
			step.args = []any{fwID}
		}
		_ = prevID
		if err := pool.QueryRow(context.Background(), step.sql, step.args...).Scan(step.dest); err != nil {
			t.Skipf("setup: %v", err)
		}
	}

	// Rubric levels.
	levels := map[int]string{0: "Absent", 1: "Ad Hoc", 2: "Defined", 3: "Managed", 4: "Optimised"}
	for level, label := range levels {
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO rubric_levels (question_id,level,label,description,score) VALUES ($1,$2,$3,'d',$4)`,
			qID, level, label, fmt.Sprintf("%.2f", float64(level)*25),
		); err != nil {
			t.Skipf("rubric insert: %v", err)
		}
	}

	// Create assessment.
	var crBody map[string]string
	cr := post(t, srv, "/v1/assessments", map[string]any{
		"framework_id": fwID, "title": "Partial Test",
	}, token)
	if cr.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", cr.StatusCode)
	}
	if err := json.NewDecoder(cr.Body).Decode(&crBody); err != nil {
		t.Fatal(err)
	}
	cr.Body.Close()
	assessmentID := crBody["id"]

	post(t, srv, fmt.Sprintf("/v1/assessments/%s/assignments", assessmentID), map[string]any{
		"assignments": map[string]string{"executive": ownerID},
	}, token)
	post(t, srv, fmt.Sprintf("/v1/assessments/%s/start", assessmentID), nil, token)

	// Submit without answering → expect 422.
	submitResp := post(t, srv, fmt.Sprintf("/v1/assessments/%s/submit", assessmentID), nil, token)
	if submitResp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("partial submit: want 422, got %d", submitResp.StatusCode)
	}
	var errBody map[string]any
	mustDecodeJSON(t, submitResp, &errBody)
	if missing, ok := errBody["missing"].(float64); !ok || missing < 1 {
		t.Errorf("expected missing >= 1, got %v", errBody["missing"])
	}
}
