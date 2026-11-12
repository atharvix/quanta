// Package router configures the HTTP request multiplexer and middleware chain using go-chi.
package router

import (
	"net/http"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	appmw "github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// NewRouter constructs a Chi HTTP router with essential enterprise middlewares and registered routes.
func NewRouter(
	cfg *config.Config,
	healthHandler *handlers.HealthHandler,
	treasuryHandler *handlers.TreasuryHandler,
	strategyHandler *handlers.StrategyHandler,
	fpHandler *handlers.FinancialProblemHandler,
	orchHandler *handlers.OrchestrationHandler,
	auditHandler *handlers.AuditHandler,
	authHandler *handlers.AuthHandler,
	riskHandler *handlers.RiskHandler,
	authMiddleware func(http.Handler) http.Handler,
) http.Handler {
	r := chi.NewRouter()

	// 0. Outermost CORS middleware to process preflights without authentication
	if cfg != nil && len(cfg.CORSAllowedOrigins) > 0 {
		r.Use(appmw.CORS(cfg.CORSAllowedOrigins))
	}

	// 1. Injects a unique X-Request-ID into each request for end-to-end tracing
	r.Use(chimw.RequestID)

	// 2. Extracts original client IP behind reverse proxies (Nginx/Traefik/Cloudflare)
	r.Use(chimw.RealIP)

	// 3. Logs incoming HTTP requests with method, path, response status, and duration
	r.Use(chimw.Logger)

	// 4. Gracefully catches unhandled panics and responds with 500 rather than crashing the process
	r.Use(chimw.Recoverer)

	// 5. Enforces a 60-second context timeout on all incoming requests
	r.Use(chimw.Timeout(60 * time.Second))

	// Root identity route (public)
	if healthHandler != nil {
		r.Get("/", healthHandler.Root)
		r.Get("/health", healthHandler.Liveness)
	}

	// API v1 route grouping
	r.Route("/api/v1", func(v1 chi.Router) {
		// Readiness check that pings database and reports dependencies (public)
		if healthHandler != nil {
			v1.Get("/health", healthHandler.DatabaseHealth)
		}

		// Public authentication endpoints
		if authHandler != nil {
			v1.Route("/auth", func(auth chi.Router) {
				auth.Post("/register", authHandler.Register)
				auth.Post("/login", authHandler.Login)
			})
		}

		// Authenticated & RBAC Protected endpoints
		v1.Group(func(protected chi.Router) {
			if authMiddleware != nil {
				protected.Use(authMiddleware)
			} else {
				protected.Use(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						handlers.WriteError(w, http.StatusUnauthorized, "unauthorized")
					})
				})
			}

			// Viewer and above (viewer, analyst, admin)
			protected.Group(func(viewer chi.Router) {
				viewer.Use(appmw.RequireRole(domain.RoleViewer, domain.RoleAnalyst, domain.RoleAdmin))

				if authHandler != nil {
					viewer.Get("/auth/me", authHandler.Me)
				}

				if treasuryHandler != nil {
					viewer.Get("/treasuries", treasuryHandler.List)
					viewer.Get("/treasuries/{id}", treasuryHandler.GetByID)
					if strategyHandler != nil {
						viewer.Get("/treasuries/{id}/strategies", strategyHandler.ListByTreasury)
					}
				}

				if strategyHandler != nil {
					viewer.Get("/strategies/{id}", strategyHandler.GetByID)
				}

				if fpHandler != nil {
					viewer.Get("/financial-problems", fpHandler.List)
					viewer.Get("/financial-problems/{id}", fpHandler.GetByID)
				}

				if orchHandler != nil {
					viewer.Get("/orchestrate/runs/{id}", orchHandler.GetRunByID)
				}
			})

			// Analyst and above (analyst, admin)
			protected.Group(func(analyst chi.Router) {
				analyst.Use(appmw.RequireRole(domain.RoleAnalyst, domain.RoleAdmin))

				if treasuryHandler != nil {
					analyst.Post("/treasuries", treasuryHandler.Create)
					if strategyHandler != nil {
						analyst.Post("/treasuries/{id}/strategies/generate", strategyHandler.Generate)
					}
				}

				if riskHandler != nil {
					analyst.Post("/treasuries/{id}/risk/analyze", riskHandler.AnalyzeRisk)
					analyst.Post("/treasuries/{id}/stress-test", riskHandler.AdversarialStressTest)
				}

				if fpHandler != nil {
					analyst.Post("/financial-problems", fpHandler.Create)
				}

				if orchHandler != nil {
					analyst.Post("/orchestrate/allocation", orchHandler.Allocate)
				}
			})

			// Admin only (admin)
			protected.Group(func(admin chi.Router) {
				admin.Use(appmw.RequireRole(domain.RoleAdmin))

				if auditHandler != nil {
					admin.Get("/audit-logs", auditHandler.List)
				}

				if authHandler != nil {
					admin.Get("/users", authHandler.ListUsers)
					admin.Patch("/users/{id}/role", authHandler.UpdateRole)
				}
			})
		})
	})

	return r
}
