package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// AuthHandler handles HTTP requests for user authentication and profile management.
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler constructs a new AuthHandler.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var input service.RegisterInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	user, err := h.authService.Register(r.Context(), input)
	if err != nil {
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			WriteValidationError(w, valErr.Details)
			return
		}
		if errors.Is(err, domain.ErrEmailAlreadyExists) {
			WriteError(w, http.StatusConflict, "email already registered")
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		WriteError(w, http.StatusInternalServerError, "failed to register user")
		return
	}

	WriteJSON(w, http.StatusCreated, user)
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var input service.LoginInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	resp, err := h.authService.Login(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			WriteError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		WriteError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	WriteJSON(w, http.StatusOK, resp)
}

// Me handles GET /api/v1/auth/me (requires authentication).
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.authService.GetCurrentUser(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		WriteError(w, http.StatusInternalServerError, "failed to retrieve profile")
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

// ListUsers handles GET /api/v1/users (admin only).
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.authService.ListUsers(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		WriteError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	WriteJSON(w, http.StatusOK, users)
}

// UpdateRole handles PATCH /api/v1/users/{id}/role (admin only).
func (h *AuthHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var input service.UpdateRoleInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	updated, err := h.authService.UpdateRole(r.Context(), id, input.Role)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidID) {
			WriteError(w, http.StatusBadRequest, "invalid user ID format")
			return
		}
		if errors.Is(err, domain.ErrInvalidRole) {
			WriteError(w, http.StatusBadRequest, "invalid role: must be admin, analyst, or viewer")
			return
		}
		if errors.Is(err, domain.ErrCannotDemoteOnlyAdmin) {
			WriteError(w, http.StatusBadRequest, "cannot demote the only admin")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, domain.ErrDatabaseUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		WriteError(w, http.StatusInternalServerError, "failed to update user role")
		return
	}

	WriteJSON(w, http.StatusOK, updated)
}
