package middleware

import (
	"net/http"
	"strings"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

// JWTAuth returns an HTTP middleware that validates Bearer tokens from the Authorization header.
// On success, authenticated user claims (sub, email, role) are injected into the request context.
// On failure (missing, invalid, or expired token), responds with 401 {"error":"unauthorized"}.
func JWTAuth(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				handlers.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			// Expect "Bearer <token>" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				handlers.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			tokenString := strings.TrimSpace(parts[1])
			if tokenString == "" {
				handlers.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			claims, err := authSvc.ValidateToken(tokenString)
			if err != nil {
				handlers.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			// Extract claims and inject into context
			sub, _ := claims["sub"].(string)
			email, _ := claims["email"].(string)
			role, _ := claims["role"].(string)

			if sub == "" || email == "" || role == "" {
				handlers.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			ctx := domain.WithUserContext(r.Context(), sub, email, domain.Role(role))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
