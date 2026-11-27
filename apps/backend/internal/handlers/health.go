package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
)

// IntelligenceChecker checks reachability of the Python intelligence service.
type IntelligenceChecker interface {
	Health(ctx context.Context) error
}

// HTTPIntelligenceChecker probes the Python intelligence service over HTTP.
type HTTPIntelligenceChecker struct {
	baseURL string
	client  *http.Client
}

// NewHTTPIntelligenceChecker creates an HTTPIntelligenceChecker for baseURL.
func NewHTTPIntelligenceChecker(baseURL string) *HTTPIntelligenceChecker {
	return &HTTPIntelligenceChecker{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 2 * time.Second},
	}
}

// Health sends GET /health to the intelligence service and verifies a 2xx response.
func (c *HTTPIntelligenceChecker) Health(ctx context.Context) error {
	if c.baseURL == "" {
		return fmt.Errorf("intelligence service URL is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	if corrID := middleware.CorrelationIDFromContext(ctx); corrID != "" {
		req.Header.Set(middleware.CorrelationIDHeader, corrID)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("intelligence service returned status %d", resp.StatusCode)
	}
	return nil
}

// DependencyStatus reports the state of a downstream dependency in readiness checks.
type DependencyStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// HealthHandler serves liveness and readiness endpoints.
type HealthHandler struct {
	db           database.Pinger
	intelligence IntelligenceChecker
}

// NewHealthHandler constructs a HealthHandler with database and intelligence checkers.
func NewHealthHandler(db database.Pinger, intelligence IntelligenceChecker) *HealthHandler {
	return &HealthHandler{
		db:           db,
		intelligence: intelligence,
	}
}

// Liveness handles GET /health. It always returns 200 OK when the HTTP process is alive.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"service":        "quanta-backend",
		"correlation_id": middleware.CorrelationIDFromContext(r.Context()),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	})
}

// Readiness handles GET /api/v1/health.
// Returns 503 SERVICE_UNAVAILABLE if PostgreSQL is unreachable.
// Returns 200 with status "degraded" if PostgreSQL is up but the Python intelligence service is unreachable.
// Returns 200 with status "ok" if both dependencies are reachable.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dbStatus := DependencyStatus{Status: "ok"}
	if h.db == nil {
		dbStatus = DependencyStatus{Status: "unavailable", Error: "database client is not configured"}
	} else if err := h.db.Ping(ctx); err != nil {
		dbStatus = DependencyStatus{Status: "unavailable", Error: err.Error()}
	}

	intelStatus := DependencyStatus{Status: "ok"}
	if h.intelligence == nil {
		intelStatus = DependencyStatus{Status: "unavailable", Error: "intelligence checker is not configured"}
	} else if err := h.intelligence.Health(ctx); err != nil {
		intelStatus = DependencyStatus{Status: "unavailable", Error: err.Error()}
	}

	dependencies := map[string]DependencyStatus{
		"postgres":     dbStatus,
		"intelligence": intelStatus,
	}

	if dbStatus.Status != "ok" {
		WriteError(
			w,
			r,
			http.StatusServiceUnavailable,
			"SERVICE_UNAVAILABLE",
			"database dependency is unavailable",
			map[string]any{"dependencies": dependencies},
		)
		return
	}

	overallStatus := "ok"
	if intelStatus.Status != "ok" {
		overallStatus = "degraded"
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"status":         overallStatus,
		"service":        "quanta-backend",
		"correlation_id": middleware.CorrelationIDFromContext(r.Context()),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"dependencies":   dependencies,
	})
}
