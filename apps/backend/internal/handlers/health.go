package handlers

import (
	"net/http"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/database"
)

// HealthHandler manages system diagnostics and dependency health checks.
type HealthHandler struct {
	pinger   database.Pinger
	aiClient clients.AIClient
}

// NewHealthHandler constructs a HealthHandler with database and AI engine checkers.
func NewHealthHandler(pinger database.Pinger, aiClient clients.AIClient) *HealthHandler {
	return &HealthHandler{
		pinger:   pinger,
		aiClient: aiClient,
	}
}

// Root handles GET /: returns service identity and version.
func (h *HealthHandler) Root(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{
		"service": "quanta-backend",
		"version": "0.1.0",
	})
}

// Liveness handles GET /health: lightweight liveness check for container orchestrators.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// DatabaseHealth handles GET /api/v1/health: readiness probe checking PostgreSQL and reporting AI engine status.
func (h *HealthHandler) DatabaseHealth(w http.ResponseWriter, r *http.Request) {
	if h.pinger == nil || h.pinger.Ping(r.Context()) != nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":   "degraded",
			"database": "down",
		})
		return
	}

	aiStatus := "up"
	if h.aiClient == nil || h.aiClient.Health(r.Context()) != nil {
		aiStatus = "down"
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"database": "up",
		"dependencies": map[string]string{
			"ai_engine": aiStatus,
		},
	})
}
