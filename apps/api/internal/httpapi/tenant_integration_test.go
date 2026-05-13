package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/YASSERRMD/Readinova/apps/api/internal/httpapi"
)

var jwtSecret = []byte("test-secret-32-bytes-long-for-test!")

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("READINOVA_DATABASE_URL")
	if dsn == "" {
		t.Skip("READINOVA_DATABASE_URL required")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func resetAndMigrate(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("READINOVA_DATABASE_URL")
	if dsn == "" {
		t.Skip("READINOVA_DATABASE_URL required")
	}

	pool := openPool(t)

	// Drop and recreate schema.
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	for _, stmt := range []string{"DROP SCHEMA public CASCADE", "CREATE SCHEMA public"} {
		if _, err := conn.Exec(context.Background(), stmt); err != nil {
			t.Fatalf("reset schema: %v", err)
		}
	}
	conn.Release()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open sql db: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}
	if err := goose.Up(db, migDir(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return pool
}

func migDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../migrations"))
}

func newServer(t *testing.T, pool *pgxpool.Pool) *httptest.Server {
	t.Helper()
	srv := httpapi.New(pool, jwtSecret)
	mux := http.NewServeMux()
	srv.Routes(mux)
	return httptest.NewServer(mux)
}

func post(t *testing.T, server *httptest.Server, path string, body any, token string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, server.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	return resp
}

func mustDecodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

// TestSignupInviteAccept tests the full org creation → invite → accept flow.
func TestSignupInviteAccept(t *testing.T) {
	pool := resetAndMigrate(t)
	srv := newServer(t, pool)
	defer srv.Close()

	// 1. Signup as owner.
	signupResp := post(t, srv, "/v1/organisations", map[string]any{
		"org_name":     "Acme Corp",
		"org_slug":     "acme-corp",
		"country_code": "GB",
		"sector":       "finance",
		"size_band":    "51-250",
		"email":        "owner@acme.com",
		"password":     "Secure#Password123",
	}, "")

	if signupResp.StatusCode != http.StatusCreated {
		t.Fatalf("signup: want 201, got %d", signupResp.StatusCode)
	}

	var signupBody map[string]string
	mustDecodeJSON(t, signupResp, &signupBody)
	ownerToken := signupBody["access_token"]
	if ownerToken == "" {
		t.Fatal("signup: no access_token in response")
	}

	// 2. Owner invites a CIO.
	invResp := post(t, srv, "/v1/invitations", map[string]any{
		"email": "cio@acme.com",
		"role":  "cio",
	}, ownerToken)

	if invResp.StatusCode != http.StatusCreated {
		t.Fatalf("invite: want 201, got %d", invResp.StatusCode)
	}

	var invBody map[string]string
	mustDecodeJSON(t, invResp, &invBody)
	invToken := invBody["token"]
	if invToken == "" {
		t.Fatal("invite: no token in response")
	}

	// 3. CIO accepts the invitation.
	acceptResp := post(t, srv, "/v1/invitations/"+invToken+"/accept", map[string]any{
		"email":    "cio@acme.com",
		"password": "AnotherSecure#123",
	}, "")

	if acceptResp.StatusCode != http.StatusOK {
		t.Fatalf("accept: want 200, got %d", acceptResp.StatusCode)
	}

	var acceptBody map[string]string
	mustDecodeJSON(t, acceptResp, &acceptBody)
	if acceptBody["role"] != "cio" {
		t.Errorf("accepted role: want cio, got %s", acceptBody["role"])
	}
	if acceptBody["access_token"] == "" {
		t.Error("accept: no access_token")
	}
}

// TestRLSIsolation verifies that a user in org A cannot read org B's members.
func TestRLSIsolation(t *testing.T) {
	pool := resetAndMigrate(t)
	srv := newServer(t, pool)
	defer srv.Close()

	// Create org A.
	r1 := post(t, srv, "/v1/organisations", map[string]any{
		"org_name": "Org A", "org_slug": "org-a",
		"country_code": "US", "sector": "tech", "size_band": "1-50",
		"email": "a@a.com", "password": "SecurePasswordA123",
	}, "")
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("signup A: %d", r1.StatusCode)
	}
	var bodyA map[string]string
	mustDecodeJSON(t, r1, &bodyA)
	orgAID := bodyA["organisation_id"]

	// Create org B.
	r2 := post(t, srv, "/v1/organisations", map[string]any{
		"org_name": "Org B", "org_slug": "org-b",
		"country_code": "US", "sector": "tech", "size_band": "1-50",
		"email": "b@b.com", "password": "SecurePasswordB123",
	}, "")
	if r2.StatusCode != http.StatusCreated {
		t.Fatalf("signup B: %d", r2.StatusCode)
	}
	var bodyB map[string]string
	mustDecodeJSON(t, r2, &bodyB)
	orgBID := bodyB["organisation_id"]

	// Directly verify RLS: user of org A cannot see org B's members.
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	// Set context to org A.
	if _, err := conn.Exec(context.Background(), "SELECT set_tenant_context($1)", orgAID); err != nil {
		t.Fatalf("set tenant A: %v", err)
	}

	var count int
	if err := conn.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM organisation_members WHERE organisation_id = $1`, orgBID,
	).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}

	if count != 0 {
		t.Errorf("RLS isolation failed: org A can see %d members of org B", count)
	}

	_ = orgBID // used above
}

// TestLoginAndRefresh verifies JWT rotation works.
func TestLoginAndRefresh(t *testing.T) {
	pool := resetAndMigrate(t)
	srv := newServer(t, pool)
	defer srv.Close()

	post(t, srv, "/v1/organisations", map[string]any{
		"org_name": "Refresh Corp", "org_slug": "refresh-corp",
		"country_code": "AU", "sector": "gov", "size_band": "51-250",
		"email": "owner@refresh.com", "password": "RefreshPassword123!",
	}, "")

	loginResp := post(t, srv, "/v1/auth/login", map[string]any{
		"email":    "owner@refresh.com",
		"password": "RefreshPassword123!",
		"org_slug": "refresh-corp",
	}, "")

	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login: want 200, got %d", loginResp.StatusCode)
	}

	// Refresh token should be in cookie.
	var refreshCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatal("no refresh_token cookie")
	}

	// Use refresh token.
	refreshReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/auth/refresh", http.NoBody)
	refreshReq.AddCookie(refreshCookie)
	refreshResp, err := http.DefaultClient.Do(refreshReq)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshResp.StatusCode != http.StatusOK {
		t.Fatalf("refresh: want 200, got %d", refreshResp.StatusCode)
	}

	var refreshBody map[string]string
	mustDecodeJSON(t, refreshResp, &refreshBody)
	if refreshBody["access_token"] == "" {
		t.Error("refresh: no access_token")
	}

	// Old token should now be invalidated — replaying it should fail.
	refreshReq2, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/auth/refresh", http.NoBody)
	refreshReq2.AddCookie(refreshCookie) // same old cookie
	oldResp, err := http.DefaultClient.Do(refreshReq2)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if oldResp.StatusCode == http.StatusOK {
		t.Error("refresh token replay should have been rejected")
	}
}
