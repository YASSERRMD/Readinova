package httpapi

import (
	"net/http"
	"time"

	"github.com/YASSERRMD/Readinova/apps/api/internal/auth"
)

// POST /v1/organisations — create org + owner user (signup).
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrgName     string `json:"org_name"`
		OrgSlug     string `json:"org_slug"`
		CountryCode string `json:"country_code"`
		Sector      string `json:"sector"`
		SizeBand    string `json:"size_band"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Email == "" || req.Password == "" || req.OrgName == "" || req.OrgSlug == "" {
		writeError(w, http.StatusUnprocessableEntity, "email, password, org_name and org_slug are required")
		return
	}
	if len(req.Password) < 12 {
		writeError(w, http.StatusUnprocessableEntity, "password must be at least 12 characters")
		return
	}

	hashed, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	var orgID, userID string
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO organisations (slug, name, country_code, sector, size_band)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		req.OrgSlug, req.OrgName, req.CountryCode, req.Sector, req.SizeBand,
	).Scan(&orgID); err != nil {
		writeError(w, http.StatusConflict, "organisation slug already taken")
		return
	}

	if err := tx.QueryRow(r.Context(),
		`INSERT INTO users (email, hashed_password) VALUES ($1,$2) RETURNING id`,
		req.Email, hashed,
	).Scan(&userID); err != nil {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}

	if _, err := tx.Exec(r.Context(),
		`INSERT INTO organisation_members (organisation_id, user_id, role) VALUES ($1,$2,'owner')`,
		orgID, userID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	if _, err := tx.Exec(r.Context(),
		`INSERT INTO audit_log (organisation_id, user_id, action, target_type, target_id)
		 VALUES ($1,$2,'signup','organisation',$1)`, orgID, userID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	accessToken, err := auth.IssueAccessToken(s.jwtSecret, userID, orgID, "owner")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"organisation_id": orgID,
		"user_id":         userID,
		"access_token":    accessToken,
	})
}

// POST /v1/auth/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		OrgSlug  string `json:"org_slug"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	var userID, hashedPw, orgID, role string
	err := s.db.QueryRow(r.Context(), `
		SELECT u.id, u.hashed_password, om.organisation_id, om.role
		FROM users u
		JOIN organisation_members om ON om.user_id = u.id
		JOIN organisations o ON o.id = om.organisation_id
		WHERE u.email = $1 AND o.slug = $2
	`, req.Email, req.OrgSlug).Scan(&userID, &hashedPw, &orgID, &role)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	ok, err := auth.VerifyPassword(req.Password, hashedPw)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Update last_login_at.
	_, _ = s.db.Exec(r.Context(), `UPDATE users SET last_login_at = now() WHERE id = $1`, userID)

	accessToken, err := auth.IssueAccessToken(s.jwtSecret, userID, orgID, role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}

	plainRefresh, hashRefresh, err := auth.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}

	if _, err := s.db.Exec(r.Context(),
		`INSERT INTO refresh_tokens (user_id, organisation_id, token_hash) VALUES ($1,$2,$3)`,
		userID, orgID, hashRefresh,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	// Set refresh token in httpOnly cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    plainRefresh,
		Path:     "/v1/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(auth.RefreshTokenTTL),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"user_id":      userID,
		"org_id":       orgID,
		"role":         role,
	})
}

// POST /v1/auth/refresh
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	tokenHash := auth.HashRefreshToken(cookie.Value)

	// Perform the full rotation atomically inside a transaction so a concurrent
	// refresh can never reuse the same token twice.
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	var userID, orgID string
	var expiresAt time.Time
	if err := tx.QueryRow(r.Context(), `
		SELECT user_id, organisation_id, expires_at
		FROM refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
		FOR UPDATE
	`, tokenHash).Scan(&userID, &orgID, &expiresAt); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Revoke old token.
	if _, err := tx.Exec(r.Context(),
		`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1`, tokenHash,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	var role string
	_ = tx.QueryRow(r.Context(),
		`SELECT role FROM organisation_members WHERE user_id = $1 AND organisation_id = $2`,
		userID, orgID,
	).Scan(&role)

	plainRefresh, hashRefresh, err := auth.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}

	if _, err := tx.Exec(r.Context(),
		`INSERT INTO refresh_tokens (user_id, organisation_id, token_hash) VALUES ($1,$2,$3)`,
		userID, orgID, hashRefresh,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	accessToken, err := auth.IssueAccessToken(s.jwtSecret, userID, orgID, role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    plainRefresh,
		Path:     "/v1/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(auth.RefreshTokenTTL),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
	})
}

// POST /v1/auth/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		// Revoke the refresh token if present (best-effort; ignore DB errors).
		tokenHash := auth.HashRefreshToken(cookie.Value)
		_, _ = s.db.Exec(r.Context(),
			`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
			tokenHash,
		)
	}

	// Clear the cookie regardless.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/v1/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)

	var email string
	if err := s.db.QueryRow(r.Context(),
		`SELECT email FROM users WHERE id = $1`, claims.UserID,
	).Scan(&email); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": claims.UserID,
		"email":   email,
		"org_id":  claims.OrgID,
		"role":    claims.Role,
	})
}

// PATCH /v1/organisations/{id}
func (s *Server) handlePatchOrg(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims.Role != "owner" && claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "owner or admin role required")
		return
	}

	var req struct {
		Name     *string `json:"name"`
		SizeBand *string `json:"size_band"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name != nil {
		if _, err := s.db.Exec(r.Context(),
			`UPDATE organisations SET name = $1, updated_at = now() WHERE id = $2`, *req.Name, claims.OrgID,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
	}
	if req.SizeBand != nil {
		if _, err := s.db.Exec(r.Context(),
			`UPDATE organisations SET size_band = $1, updated_at = now() WHERE id = $2`, *req.SizeBand, claims.OrgID,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
