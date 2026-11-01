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

	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/atharvix/quanta/apps/backend/internal/router"
)

func main() {
	// 1. Load environment configuration with production defaults
	cfg := config.Load()
	log.Printf("[INFO] Initializing QUANTA Backend Service with %s", cfg.String())

	// 2. Attempt connection to PostgreSQL (pgxpool)
	// If PostgreSQL is offline, log a warning and continue starting the HTTP server in degraded mode.
	initCtx, initCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer initCancel()

	db, err := database.NewPool(initCtx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("[WARN] PostgreSQL connection failed at bootstrap: %v. Running in degraded mode without database.", err)
	} else {
		defer db.Close()
		log.Println("[INFO] PostgreSQL connection pool initialized and verified successfully.")
	}

	// 3. Initialize HTTP handlers and router
	healthHandler := handlers.NewHealthHandler(db)
	appRouter := router.NewRouter(cfg, healthHandler)

	// 4. Configure robust HTTP server with enterprise timeouts
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      appRouter,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 5. Start listening asynchronously in a goroutine
	go func() {
		log.Printf("[INFO] QUANTA Backend HTTP server listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server failed unexpectedly: %v", err)
		}
	}()

	// 6. Graceful shutdown on SIGINT / SIGTERM
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, syscall.SIGTERM)
	<-stopSignal

	log.Println("[INFO] Received termination signal. Initiating graceful shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] HTTP server forced shutdown error: %v", err)
	} else {
		log.Println("[INFO] HTTP server stopped cleanly.")
	}
}
