package middleware

import (
	"net/http"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
)

// RequireRole returns an HTTP middleware that verifies the authenticated caller has one of the allowed roles.
// If the user context is missing or unauthenticated, returns 401 {"error":"unauthorized"}.
// If the user role is not authorized, returns 403 {"error":"forbidden"}.
func RequireRole(allowedRoles ...domain.Role) func(http.Handler) http.Handler {
	allowedMap := make(map[domain.Role]bool, len(allowedRoles))
	for _, r := range allowedRoles {
		allowedMap[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := domain.RoleFromContext(r.Context())
			if userRole == "" {
				handlers.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			if !allowedMap[userRole] {
				handlers.WriteError(w, http.StatusForbidden, "forbidden")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
