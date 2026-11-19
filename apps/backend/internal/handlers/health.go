package handlers

import (
	"net/http"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/database"
)

// HealthHandler manages system diagnostics and dependency health checks.
type HealthHandler struct {
	pinger    database.Pinger
	aiClient  clients.AIClient
	optClient clients.OptimizationClient
}

// NewHealthHandler constructs a HealthHandler with database and AI engine checkers,
// with optional support for Member 2's Optimization Engine.
func NewHealthHandler(pinger database.Pinger, aiClient clients.AIClient, optClient ...clients.OptimizationClient) *HealthHandler {
	var oc clients.OptimizationClient
	if len(optClient) > 0 {
		oc = optClient[0]
	}
	return &HealthHandler{
		pinger:    pinger,
		aiClient:  aiClient,
		optClient: oc,
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

	deps := map[string]string{
		"ai_engine": aiStatus,
	}
	if h.optClient != nil {
		optStatus := "up"
		if h.optClient.Health(r.Context()) != nil {
			optStatus = "down"
		}
		deps["optimization_engine"] = optStatus
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"database":     "up",
		"dependencies": deps,
	})
}
