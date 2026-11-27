package handlers

import (
	"net/http"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

type AuthHandler struct {
	authSvc *service.AuthService
}

func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	if req.Email == "" || req.Password == "" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "email and password are required")
		return
	}

	corrID := middleware.CorrelationIDFromContext(r.Context())
	res, err := h.authSvc.Login(r.Context(), req.Email, req.Password, corrID)
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"access_token": res.AccessToken,
		"token_type":   res.TokenType,
		"expires_at":   res.ExpiresAt.UTC().Format(time.RFC3339),
		"user": map[string]any{
			"id":        res.User.ID,
			"email":     res.User.Email,
			"full_name": res.User.FullName,
			"is_active": res.User.IsActive,
		},
		"memberships": serializeMemberships(res.Memberships),
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.AuthenticatedUserFromContext(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	user, memberships, err := h.authSvc.Me(r.Context(), userCtx.UserID)
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":        user.ID,
			"email":     user.Email,
			"full_name": user.FullName,
			"is_active": user.IsActive,
		},
		"memberships": serializeMemberships(memberships),
	})
}

func serializeMemberships(items []domain.OrganizationMembership) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, m := range items {
		out = append(out, map[string]any{
			"id":              m.ID,
			"organization_id": m.OrganizationID,
			"role":            m.Role,
			"status":          m.Status,
		})
	}
	return out
}
