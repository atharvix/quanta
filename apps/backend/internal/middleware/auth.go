package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/auth"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

const (
	authUserContextKey   contextKey = "auth_user"
	orgContextContextKey contextKey = "org_context"
)

// AuthenticatedUser holds verified identity claims from a Bearer JWT.
type AuthenticatedUser struct {
	UserID uuid.UUID
	Email  string
}

// OrgContext holds the verified active organization and RBAC role for a request.
type OrgContext struct {
	OrganizationID uuid.UUID
	Role           domain.MemberRole
}

// AuthenticatedUserFromContext retrieves the authenticated user from ctx.
func AuthenticatedUserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	if ctx == nil {
		return AuthenticatedUser{}, false
	}
	u, ok := ctx.Value(authUserContextKey).(AuthenticatedUser)
	return u, ok
}

// OrgContextFromContext retrieves the verified organization context from ctx.
func OrgContextFromContext(ctx context.Context) (OrgContext, bool) {
	if ctx == nil {
		return OrgContext{}, false
	}
	oc, ok := ctx.Value(orgContextContextKey).(OrgContext)
	return oc, ok
}

// RequireAuth validates the Bearer JWT and stores AuthenticatedUser in request context.
func RequireAuth(tokens *auth.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeMiddlewareError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid Authorization header")
				return
			}
			rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			claims, err := tokens.VerifyAccessToken(rawToken)
			if err != nil {
				writeMiddlewareError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired access token")
				return
			}

			ctx := context.WithValue(r.Context(), authUserContextKey, AuthenticatedUser{
				UserID: claims.Subject,
				Email:  claims.Email,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireOrgPermission enforces active OrganizationMembership and role permission for {orgId}.
// Access is denied by default on invalid orgId, missing membership, suspended membership, or insufficient role.
func RequireOrgPermission(orgRepo repository.OrganizationRepository, perm auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := AuthenticatedUserFromContext(r.Context())
			if !ok {
				writeMiddlewareError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
				return
			}

			rawOrgID := strings.TrimSpace(chi.URLParam(r, "orgId"))
			orgID, err := uuid.Parse(rawOrgID)
			if err != nil {
				writeMiddlewareError(w, r, http.StatusBadRequest, "INVALID_ORGANIZATION_ID", "invalid organization ID")
				return
			}

			membership, err := orgRepo.GetMembership(r.Context(), orgID, user.UserID)
			if err != nil || membership.Status != domain.OrgStatusActive {
				writeMiddlewareError(w, r, http.StatusForbidden, "FORBIDDEN", "active organization membership required")
				return
			}

			if !auth.HasPermission(membership.Role, perm) {
				writeMiddlewareError(w, r, http.StatusForbidden, "FORBIDDEN", "insufficient organization permissions")
				return
			}

			ctx := context.WithValue(r.Context(), orgContextContextKey, OrgContext{
				OrganizationID: orgID,
				Role:           membership.Role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeMiddlewareError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":           code,
			"message":        message,
			"correlation_id": CorrelationIDFromContext(r.Context()),
		},
	})
}
