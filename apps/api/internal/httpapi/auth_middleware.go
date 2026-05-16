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

// claimsFromCtx retrieves auth claims from context.
// It returns nil when called outside a withAuth-wrapped handler; callers must
// check for nil if they are not guaranteed to be behind withAuth.
func claimsFromCtx(r *http.Request) *auth.Claims {
	v := r.Context().Value(claimsKey{})
	if v == nil {
		return nil
	}
	c, _ := v.(*auth.Claims)
	return c
}
