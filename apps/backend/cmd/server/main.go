package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/auth"
	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/atharvix/quanta/apps/backend/internal/intelligence"
	"github.com/atharvix/quanta/apps/backend/internal/repository/postgres"
	"github.com/atharvix/quanta/apps/backend/internal/router"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid backend configuration", "error", err)
		os.Exit(1)
	}

	tokenSvc, err := auth.NewTokenService(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTTTL)
	if err != nil {
		logger.Error("failed to initialize token service", "error", err)
		os.Exit(1)
	}

	logger.Info(
		"starting quanta backend",
		"env", cfg.Environment,
		"port", cfg.Port,
		"database_url", cfg.MaskedDatabaseURL(),
		"intelligence_url", cfg.IntelligenceURL,
	)

	db := database.NewDB(cfg.DatabaseURL)
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.Connect(connectCtx); err != nil {
		// Allow process startup so GET /health stays live and GET /api/v1/health reports 503 clearly.
		logger.Warn("postgres connection unavailable at startup", "error", err)
	} else {
		logger.Info("postgres connection established")
	}
	cancelConnect()
	defer db.Close()

	pool := db.Pool()
	orgRepo := postgres.NewOrganizationRepository(pool)
	userRepo := postgres.NewUserRepository(pool)
	treasuryRepo := postgres.NewTreasuryRepository(pool)
	accountRepo := postgres.NewAccountRepository(pool)
	ledgerRepo := postgres.NewLedgerRepository(pool)
	policyRepo := postgres.NewLiquidityPolicyRepository(pool)
	cpRepo := postgres.NewCounterpartyRepository(pool)
	runRepo := postgres.NewOptimizationRunRepository(pool)
	proposalRepo := postgres.NewProposalRepository(pool)
	decisionRepo := postgres.NewApprovalDecisionRepository(pool)
	auditRepo := postgres.NewAuditRepository(pool)
	txMgr := postgres.NewTxManager(pool)

	intelClient := intelligence.NewHTTPClient(cfg.IntelligenceURL, 15*time.Second)

	authSvc := service.NewAuthService(userRepo, auditRepo, tokenSvc)
	accountSvc := service.NewAccountService(treasuryRepo, accountRepo, ledgerRepo, auditRepo)
	ledgerSvc := service.NewLedgerService(txMgr)
	policySvc := service.NewPolicyService(treasuryRepo, policyRepo, auditRepo)
	cpSvc := service.NewCounterpartyService(treasuryRepo, cpRepo, auditRepo)
	optSvc := service.NewOptimizationService(treasuryRepo, accountRepo, policyRepo, cpRepo, runRepo, auditRepo, intelClient)
	proposalSvc := service.NewProposalService(txMgr, runRepo, proposalRepo, decisionRepo, treasuryRepo, accountRepo, policyRepo, cpRepo, auditRepo)

	intelChecker := handlers.NewHTTPIntelligenceChecker(cfg.IntelligenceURL)
	httpRouter := router.NewRouter(cfg, router.Dependencies{
		Health:       handlers.NewHealthHandler(db, intelChecker),
		Auth:         handlers.NewAuthHandler(authSvc),
		Treasury:     handlers.NewTreasuryHandler(accountSvc, ledgerSvc),
		Policy:       handlers.NewPolicyHandler(policySvc),
		Counterparty: handlers.NewCounterpartyHandler(cpSvc),
		Intelligence: handlers.NewIntelligenceHandler(optSvc),
		Proposal:     handlers.NewProposalHandler(proposalSvc),
		Audit:        handlers.NewAuditHandler(auditRepo),
		Tokens:       tokenSvc,
		OrgRepo:      orgRepo,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpRouter,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stop:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		logger.Error("http server failed", "error", err)
		os.Exit(1)
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("quanta backend stopped cleanly")
}
