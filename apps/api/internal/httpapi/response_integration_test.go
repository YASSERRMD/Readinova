package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// putJSON sends a PUT request with a JSON body and an optional bearer token.
func putJSON(t *testing.T, srv *httptest.Server, path string, body any, token string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	return resp
}

// deleteReq sends a DELETE request with an optional bearer token.
func deleteReq(t *testing.T, srv *httptest.Server, path string, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+path, http.NoBody)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	return resp
}

// setupResponseFixture creates a full assessment fixture ready for responses:
// org, owner, framework (1 question), assessment, assignment, started.
// Returns (srv, token, assessmentID, questionSlug).
func setupResponseFixture(t *testing.T) (srv *httptest.Server, token, assessmentID, questionSlug string) {
	t.Helper()
	pool := resetAndMigrate(t)
	srv = newServer(t, pool)

	signupResp := post(t, srv, "/v1/organisations", map[string]any{
		"org_name": "Resp Corp", "org_slug": "resp-corp",
		"country_code": "GB", "sector": "finance", "size_band": "1-50",
		"email": "owner@resp.com", "password": "RespPass123!",
	}, "")
	var signupBody map[string]string
	mustDecodeJSON(t, signupResp, &signupBody)
	token = signupBody["access_token"]
	ownerID := signupBody["user_id"]

	// Build framework hierarchy.
	var fwID, dimID, sdID, indID, qID string
	type step struct {
		dest *string
		sql  string
		arg  any
	}
	steps := []step{
		{&fwID, `INSERT INTO frameworks (slug,name,version_major,version_minor,status) VALUES ('resp-fw','Resp FW',1,0,'published') RETURNING id`, nil},
		{&dimID, `INSERT INTO dimensions (framework_id,slug,name,description,default_weight,display_order) VALUES ($1,'r_dim','R Dim','d','1.0000',1) RETURNING id`, &fwID},
		{&sdID, `INSERT INTO sub_dimensions (dimension_id,slug,name,description,weight_within_dimension,display_order) VALUES ($1,'r_sd','R SD','d','1.0000',1) RETURNING id`, &dimID},
		{&indID, `INSERT INTO indicators (sub_dimension_id,slug,name,description,display_order) VALUES ($1,'r_ind','R Ind','d',1) RETURNING id`, &sdID},
		{&qID, `INSERT INTO questions (indicator_id,slug,prompt,target_role,display_order,regulatory_references) VALUES ($1,'r_q','Question?','executive',1,'{}') RETURNING id`, &indID},
	}
	for i, s := range steps {
		var arg any
		if s.arg != nil {
			arg = *(s.arg.(*string))
		}
		if err := pool.QueryRow(context.Background(), s.sql, arg).Scan(s.dest); err != nil {
			t.Skipf("fixture step %d: %v", i, err)
		}
	}
	questionSlug = "r_q"

	// Insert rubric levels 0-4.
	labels := map[int]string{0: "Absent", 1: "Ad Hoc", 2: "Defined", 3: "Managed", 4: "Optimised"}
	for lvl, lbl := range labels {
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO rubric_levels (question_id,level,label,description,score) VALUES ($1,$2,$3,'d',$4)`,
			qID, lvl, lbl, fmt.Sprintf("%.2f", float64(lvl)*25),
		); err != nil {
			t.Skipf("rubric insert: %v", err)
		}
	}

	// Create assessment.
	cr := post(t, srv, "/v1/assessments", map[string]any{
		"framework_id": fwID, "title": "Resp Test",
	}, token)
	if cr.StatusCode != http.StatusCreated {
		t.Fatalf("create assessment: %d", cr.StatusCode)
	}
	var crBody map[string]string
	mustDecodeJSON(t, cr, &crBody)
	assessmentID = crBody["id"]

	// Assign and start.
	post(t, srv, fmt.Sprintf("/v1/assessments/%s/assignments", assessmentID), map[string]any{
		"assignments": map[string]string{"executive": ownerID},
	}, token)
	startResp := post(t, srv, fmt.Sprintf("/v1/assessments/%s/start", assessmentID), nil, token)
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("start assessment: %d", startResp.StatusCode)
	}
	return srv, token, assessmentID, questionSlug
}

// TestUpsertResponseHappyPath verifies that a valid PUT creates then updates a response.
func TestUpsertResponseHappyPath(t *testing.T) {
	srv, token, assessmentID, slug := setupResponseFixture(t)
	defer srv.Close()

	url := fmt.Sprintf("/v1/assessments/%s/responses/%s", assessmentID, slug)

	r1 := putJSON(t, srv, url, map[string]any{"level": 2}, token)
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("upsert 1: want 200, got %d", r1.StatusCode)
	}
	var b1 map[string]any
	mustDecodeJSON(t, r1, &b1)
	if b1["level"].(float64) != 2 {
		t.Errorf("level: want 2, got %v", b1["level"])
	}

	r2 := putJSON(t, srv, url, map[string]any{"level": 4}, token)
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("upsert 2: want 200, got %d", r2.StatusCode)
	}
	var b2 map[string]any
	mustDecodeJSON(t, r2, &b2)
	if b2["level"].(float64) != 4 {
		t.Errorf("level after update: want 4, got %v", b2["level"])
	}
}

// TestUpsertResponseInvalidLevel verifies that levels outside 0-4 return 422.
func TestUpsertResponseInvalidLevel(t *testing.T) {
	srv, token, assessmentID, slug := setupResponseFixture(t)
	defer srv.Close()

	url := fmt.Sprintf("/v1/assessments/%s/responses/%s", assessmentID, slug)
	r := putJSON(t, srv, url, map[string]any{"level": 5}, token)
	if r.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("level=5: want 422, got %d", r.StatusCode)
	}
}

// TestUpsertResponseUnknownQuestion returns 404 for a non-existent question slug.
func TestUpsertResponseUnknownQuestion(t *testing.T) {
	srv, token, assessmentID, _ := setupResponseFixture(t)
	defer srv.Close()

	url := fmt.Sprintf("/v1/assessments/%s/responses/no_such_q", assessmentID)
	r := putJSON(t, srv, url, map[string]any{"level": 1}, token)
	if r.StatusCode != http.StatusNotFound {
		t.Errorf("unknown question: want 404, got %d", r.StatusCode)
	}
}

// TestResponseImmutableAfterSubmit verifies 412 on upsert/delete after submit.
func TestResponseImmutableAfterSubmit(t *testing.T) {
	srv, token, assessmentID, slug := setupResponseFixture(t)
	defer srv.Close()

	url := fmt.Sprintf("/v1/assessments/%s/responses/%s", assessmentID, slug)

	r := putJSON(t, srv, url, map[string]any{"level": 3}, token)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("upsert before submit: %d", r.StatusCode)
	}

	submitResp := post(t, srv, fmt.Sprintf("/v1/assessments/%s/submit", assessmentID), nil, token)
	if submitResp.StatusCode != http.StatusOK {
		t.Fatalf("submit: want 200, got %d", submitResp.StatusCode)
	}

	r2 := putJSON(t, srv, url, map[string]any{"level": 1}, token)
	if r2.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("upsert after submit: want 412, got %d", r2.StatusCode)
	}

	del := deleteReq(t, srv, url, token)
	if del.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("delete after submit: want 412, got %d", del.StatusCode)
	}
}

// TestListResponses verifies the list endpoint returns responses with role filter.
func TestListResponses(t *testing.T) {
	srv, token, assessmentID, slug := setupResponseFixture(t)
	defer srv.Close()

	putURL := fmt.Sprintf("/v1/assessments/%s/responses/%s", assessmentID, slug)
	r := putJSON(t, srv, putURL, map[string]any{"level": 3, "free_text": "some text"}, token)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("upsert: %d", r.StatusCode)
	}

	listURL := fmt.Sprintf("/v1/assessments/%s/responses", assessmentID)
	listReq, _ := http.NewRequest(http.MethodGet, srv.URL+listURL, http.NoBody)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list: want 200, got %d", listResp.StatusCode)
	}
	var list []map[string]any
	mustDecodeJSON(t, listResp, &list)
	if len(list) != 1 {
		t.Fatalf("list len: want 1, got %d", len(list))
	}
	if list[0]["question_slug"] != slug {
		t.Errorf("slug: want %s, got %v", slug, list[0]["question_slug"])
	}

	// Filter by wrong role → empty list.
	listReq2, _ := http.NewRequest(http.MethodGet, srv.URL+listURL+"?role=cio", http.NoBody)
	listReq2.Header.Set("Authorization", "Bearer "+token)
	listResp2, err := http.DefaultClient.Do(listReq2)
	if err != nil {
		t.Fatalf("list request (role filter): %v", err)
	}
	var list2 []map[string]any
	mustDecodeJSON(t, listResp2, &list2)
	if len(list2) != 0 {
		t.Errorf("filtered list: want 0, got %d", len(list2))
	}
}

// TestDeleteResponse verifies a response can be deleted while in_progress.
func TestDeleteResponse(t *testing.T) {
	srv, token, assessmentID, slug := setupResponseFixture(t)
	defer srv.Close()

	url := fmt.Sprintf("/v1/assessments/%s/responses/%s", assessmentID, slug)

	r := putJSON(t, srv, url, map[string]any{"level": 2}, token)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("upsert: %d", r.StatusCode)
	}

	del := deleteReq(t, srv, url, token)
	if del.StatusCode != http.StatusNoContent {
		t.Errorf("delete: want 204, got %d", del.StatusCode)
	}

	del2 := deleteReq(t, srv, url, token)
	if del2.StatusCode != http.StatusNotFound {
		t.Errorf("double delete: want 404, got %d", del2.StatusCode)
	}
}
