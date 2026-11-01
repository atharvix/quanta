// Package router configures the HTTP request multiplexer and middleware chain using go-chi.
package router

import (
	"net/http"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter constructs a Chi HTTP router with essential enterprise middlewares and registered routes.
func NewRouter(cfg *config.Config, healthHandler *handlers.HealthHandler, treasuryHandler *handlers.TreasuryHandler) http.Handler {
	r := chi.NewRouter()

	// 1. Injects a unique X-Request-ID into each request for end-to-end tracing
	r.Use(middleware.RequestID)

	// 2. Extracts original client IP behind reverse proxies (Nginx/Traefik/Cloudflare)
	r.Use(middleware.RealIP)

	// 3. Logs incoming HTTP requests with method, path, response status, and duration
	r.Use(middleware.Logger)

	// 4. Gracefully catches unhandled panics and responds with 500 rather than crashing the process
	r.Use(middleware.Recoverer)

	// 5. Enforces a 30-second context timeout on all incoming requests
	r.Use(middleware.Timeout(30 * time.Second))

	// Root identity route
	r.Get("/", healthHandler.Root)

	// High-performance container liveness check (port 8080/health)
	r.Get("/health", healthHandler.Liveness)

	// API v1 route grouping
	r.Route("/api/v1", func(v1 chi.Router) {
		// Readiness check that pings database (port 8080/api/v1/health)
		v1.Get("/health", healthHandler.DatabaseHealth)

		// Corporate treasury endpoints
		if treasuryHandler != nil {
			v1.Route("/treasuries", func(tr chi.Router) {
				tr.Get("/", treasuryHandler.List)
				tr.Post("/", treasuryHandler.Create)
				tr.Get("/{id}", treasuryHandler.GetByID)
			})
		}
	})

	return r
}
