// Package main is the entrypoint for the QUANTA Go Backend service.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/clients"
	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
	"github.com/atharvix/quanta/apps/backend/internal/router"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

func main() {
	// 1. Load environment configuration with production defaults
	cfg := config.Load()
	if cfg.IsDefaultJWTSecret() {
		log.Println("[WARN] Using default insecure JWT_SECRET (dev-only-change-me-please). Do NOT use in production!")
	}
	log.Printf("[INFO] Initializing QUANTA Backend Service with %s", cfg.String())

	// 2. Initialize thread-safe database container with reconnection support
	db := database.NewDB(cfg.DatabaseURL)
	defer db.Close()

	// Context for graceful shutdown signal propagation
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initial bootstrap connection attempt (automatically executes migrations on connect)
	initCtx, initCancel := context.WithTimeout(ctx, 5*time.Second)
	if err := db.Connect(initCtx); err != nil {
		log.Printf("[WARN] PostgreSQL connection failed at bootstrap: %v. Running in degraded mode without database.", err)
	} else {
		log.Println("[INFO] PostgreSQL connection pool initialized and migrations verified successfully.")
	}
	initCancel()

	// 3. Start background retry loop to automatically reconnect if DB is offline or recovers
	db.StartBackgroundRetry(ctx, 5*time.Second)

	// 4. Initialize HTTP client for upstream AI Engine
	aiClient := clients.NewHTTPAIClient(cfg.AIEngineURL)

	// 5. Initialize clean architecture layers
	auditRepo := repository.NewPgxAuditRepository(db)
	auditSvc := service.NewAuditService(auditRepo)

	treasuryRepo := repository.NewPgxTreasuryRepository(db)
	treasuryService := service.NewTreasuryService(treasuryRepo, auditSvc)
	treasuryHandler := handlers.NewTreasuryHandler(treasuryService)

	strategyRepo := repository.NewPgxStrategyRepository(db)
	strategyService := service.NewStrategyService(treasuryRepo, strategyRepo, aiClient, auditSvc)
	strategyHandler := handlers.NewStrategyHandler(strategyService)

	fpRepo := repository.NewPgxFinancialProblemRepository(db)
	fpService := service.NewFinancialProblemService(treasuryRepo, fpRepo, strategyRepo, aiClient, auditSvc)
	fpHandler := handlers.NewFinancialProblemHandler(fpService)

	orchRepo := repository.NewPgxOrchestrationRepository(db)
	orchService := service.NewOrchestrationService(treasuryRepo, orchRepo, strategyRepo, aiClient, auditSvc)
	orchHandler := handlers.NewOrchestrationHandler(orchService)

	auditHandler := handlers.NewAuditHandler(auditSvc)
	healthHandler := handlers.NewHealthHandler(db, aiClient)

	riskService := service.NewRiskService(treasuryRepo, aiClient, auditSvc)
	riskHandler := handlers.NewRiskHandler(riskService)

	userRepo := repository.NewPgxUserRepository(db)
	authSvc := service.NewAuthService(userRepo, auditSvc, cfg)
	authHandler := handlers.NewAuthHandler(authSvc)
	authMiddleware := middleware.JWTAuth(authSvc)

	appRouter := router.NewRouter(
		cfg,
		healthHandler,
		treasuryHandler,
		strategyHandler,
		fpHandler,
		orchHandler,
		auditHandler,
		authHandler,
		riskHandler,
		authMiddleware,
	)

	// 6. Configure robust HTTP server with enterprise timeouts
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      appRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 7. Start listening asynchronously in a goroutine
	go func() {
		log.Printf("[INFO] QUANTA Backend HTTP server listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server failed unexpectedly: %v", err)
		}
	}()

	// 8. Wait for termination signal
	<-ctx.Done()
	log.Println("[INFO] Received termination signal. Initiating graceful shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] HTTP server forced shutdown error: %v", err)
	} else {
		log.Println("[INFO] HTTP server stopped cleanly.")
	}
}
