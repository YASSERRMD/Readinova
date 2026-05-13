// Package tenant provides middleware that sets the PostgreSQL row-level
// security context per request.
package tenant

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey struct{}

// OrgIDFromContext returns the organisation ID stored in the context.
func OrgIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKey{}).(string); ok {
		return v
	}
	return ""
}

// WithOrgID stores the organisation ID in the context.
func WithOrgID(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, contextKey{}, orgID)
}

// SetTenantContext executes `SELECT set_tenant_context($1)` on the given pgx
// connection pool connection so that RLS policies filter rows to the org.
func SetTenantContext(ctx context.Context, pool *pgxpool.Pool, orgID string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	_, err = conn.Exec(ctx, "SELECT set_tenant_context($1)", orgID)
	return err
}

// RequireAuth is an HTTP middleware that extracts the org ID from the
// X-Tenant-Org header (set by upstream auth middleware) and stores it in the
// request context. Returns 401 if the header is absent.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get("X-Tenant-Org")
		if orgID == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ctx := WithOrgID(r.Context(), orgID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
