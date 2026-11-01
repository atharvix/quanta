package handlers

import (
	"net/http"

	"github.com/atharvix/quanta/apps/backend/internal/database"
)

// HealthHandler manages health and diagnostic endpoints.
type HealthHandler struct {
	pinger database.Pinger
}

// NewHealthHandler constructs a HealthHandler with a database pinger.
// The pinger can be nil if the database was unavailable during application bootstrap.
func NewHealthHandler(pinger database.Pinger) *HealthHandler {
	return &HealthHandler{pinger: pinger}
}

// Root handles GET /: returns service identity and version.
func (h *HealthHandler) Root(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{
		"service": "quanta-backend",
		"version": "0.1.0",
	})
}

// Liveness handles GET /health: lightweight liveness check for container orchestrators (Docker/K8s).
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// DatabaseHealth handles GET /api/v1/health: readiness probe that pings PostgreSQL.
// If the database is nil or unreachable, returns 503 Service Unavailable without leaking internal errors.
func (h *HealthHandler) DatabaseHealth(w http.ResponseWriter, r *http.Request) {
	if h.pinger == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":   "degraded",
			"database": "down",
		})
		return
	}

	if err := h.pinger.Ping(r.Context()); err != nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":   "degraded",
			"database": "down",
		})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"status":   "ok",
		"database": "up",
	})
}
