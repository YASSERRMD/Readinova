package httpapi

import (
	"net/http"
	"time"

	"github.com/YASSERRMD/Readinova/apps/api/internal/auth"
)

// POST /v1/invitations
func (s *Server) handleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	if claims.Role != "owner" && claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "owner or admin role required")
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Email == "" || req.Role == "" {
		writeError(w, http.StatusUnprocessableEntity, "email and role are required")
		return
	}

	validRoles := map[string]bool{"owner": true, "admin": true, "executive": true, "cio": true, "risk": true, "ops": true, "viewer": true}
	if !validRoles[req.Role] {
		writeError(w, http.StatusUnprocessableEntity, "invalid role")
		return
	}

	var invID, token string
	if err := s.db.QueryRow(r.Context(),
		`INSERT INTO invitations (organisation_id, email, role)
		 VALUES ($1,$2,$3) RETURNING id, token`,
		claims.OrgID, req.Email, req.Role,
	).Scan(&invID, &token); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	_, _ = s.db.Exec(r.Context(),
		`INSERT INTO audit_log (organisation_id, user_id, action, target_type, target_id, metadata)
		 VALUES ($1,$2,'invite_sent','invitation',$3,$4::jsonb)`,
		claims.OrgID, claims.UserID, invID,
		`{"email":"`+req.Email+`","role":"`+req.Role+`"}`,
	)

	writeJSON(w, http.StatusCreated, map[string]any{
		"invitation_id": invID,
		"token":         token,
	})
}

// POST /v1/invitations/{token}/accept
func (s *Server) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusUnprocessableEntity, "email and password required")
		return
	}
	if len(req.Password) < 12 {
		writeError(w, http.StatusUnprocessableEntity, "password must be at least 12 characters")
		return
	}

	var invID, orgID, invRole string
	var expiresAt time.Time
	if err := s.db.QueryRow(r.Context(), `
		SELECT id, organisation_id, role, expires_at
		FROM invitations
		WHERE token = $1 AND accepted_at IS NULL AND expires_at > now()
	`, token).Scan(&invID, &orgID, &invRole, &expiresAt); err != nil {
		writeError(w, http.StatusNotFound, "invitation not found or expired")
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

	var userID string
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO users (email, hashed_password) VALUES ($1,$2) RETURNING id`,
		req.Email, hashed,
	).Scan(&userID); err != nil {
		// User may already exist — look them up instead.
		if err2 := tx.QueryRow(r.Context(),
			`SELECT id FROM users WHERE email = $1`, req.Email,
		).Scan(&userID); err2 != nil {
			writeError(w, http.StatusConflict, "email already in use by another account")
			return
		}
	}

	if _, err := tx.Exec(r.Context(),
		`INSERT INTO organisation_members (organisation_id, user_id, role) VALUES ($1,$2,$3)
		 ON CONFLICT (organisation_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, userID, invRole,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	if _, err := tx.Exec(r.Context(),
		`UPDATE invitations SET accepted_at = now() WHERE id = $1`, invID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	if _, err := tx.Exec(r.Context(),
		`INSERT INTO audit_log (organisation_id, user_id, action, target_type, target_id)
		 VALUES ($1,$2,'invite_accepted','invitation',$3)`, orgID, userID, invID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	accessToken, err := auth.IssueAccessToken(s.jwtSecret, userID, orgID, invRole)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":      userID,
		"org_id":       orgID,
		"role":         invRole,
		"access_token": accessToken,
	})
}
