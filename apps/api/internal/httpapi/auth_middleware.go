package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/YASSERRMD/Readinova/apps/api/internal/auth"
)

type claimsKey struct{}

// withAuth wraps a handler with JWT authentication.
func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		if !strings.HasPrefix(bearer, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		tokenStr := strings.TrimPrefix(bearer, "Bearer ")
		claims, err := auth.ParseAccessToken(s.jwtSecret, tokenStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey{}, claims)
		next(w, r.WithContext(ctx))
	}
}

// claimsFromCtx retrieves auth claims from context; panics if missing (call
// only from handlers wrapped by withAuth).
func claimsFromCtx(r *http.Request) *auth.Claims {
	return r.Context().Value(claimsKey{}).(*auth.Claims)
}
