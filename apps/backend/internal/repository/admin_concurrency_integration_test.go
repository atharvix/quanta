//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
)

func TestAdminConcurrency_PreventZeroAdmins(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://quanta_admin:quanta_secure_dev_password@localhost:5432/quanta_db?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.NewPool(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping integration test: PostgreSQL unavailable: %v", err)
	}
	defer db.Close()

	repo := NewPgxUserRepository(db)

	// Clean test harness: Create two dedicated admin accounts for this concurrency test
	ts := time.Now().UnixNano()
	adminA, err := repo.Create(ctx, domain.User{
		Email:        fmt.Sprintf("admin_concurrent_a_%d@quanta.local", ts),
		PasswordHash: "mock_bcrypt_hash_placeholder_12345",
		FullName:     "Concurrent Admin A",
		Role:         domain.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("failed to create admin A: %v", err)
	}
	defer func() {
		if db.Pool() != nil {
			_, _ = db.Pool().Exec(context.Background(), "DELETE FROM users WHERE id = $1", adminA.ID)
		}
	}()

	adminB, err := repo.Create(ctx, domain.User{
		Email:        fmt.Sprintf("admin_concurrent_b_%d@quanta.local", ts),
		PasswordHash: "mock_bcrypt_hash_placeholder_12345",
		FullName:     "Concurrent Admin B",
		Role:         domain.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("failed to create admin B: %v", err)
	}
	defer func() {
		if db.Pool() != nil {
			_, _ = db.Pool().Exec(context.Background(), "DELETE FROM users WHERE id = $1", adminB.ID)
		}
	}()

	// Barrier to trigger simultaneous demotions
	startBarrier := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	var successCount int32
	var rejectedCount int32
	var otherErrCount int32

	// Goroutine 1: Demote Admin A to viewer
	go func() {
		defer wg.Done()
		<-startBarrier
		opCtx, opCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer opCancel()

		_, demoteErr := repo.UpdateRole(opCtx, adminA.ID, domain.RoleViewer)
		if demoteErr == nil {
			atomic.AddInt32(&successCount, 1)
		} else if errors.Is(demoteErr, domain.ErrCannotDemoteOnlyAdmin) {
			atomic.AddInt32(&rejectedCount, 1)
		} else {
			atomic.AddInt32(&otherErrCount, 1)
		}
	}()

	// Goroutine 2: Demote Admin B to viewer
	go func() {
		defer wg.Done()
		<-startBarrier
		opCtx, opCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer opCancel()

		_, demoteErr := repo.UpdateRole(opCtx, adminB.ID, domain.RoleViewer)
		if demoteErr == nil {
			atomic.AddInt32(&successCount, 1)
		} else if errors.Is(demoteErr, domain.ErrCannotDemoteOnlyAdmin) {
			atomic.AddInt32(&rejectedCount, 1)
		} else {
			atomic.AddInt32(&otherErrCount, 1)
		}
	}()

	// Release both goroutines at the exact same instant
	close(startBarrier)
	wg.Wait()

	if otherErrCount > 0 {
		t.Fatalf("unexpected operational error during concurrency test: %d errors encountered", otherErrCount)
	}

	// Invariant Check: The database must retain at least one administrator
	remainingAdmins, err := repo.CountAdmins(ctx)
	if err != nil {
		t.Fatalf("failed to count remaining admins: %v", err)
	}

	if remainingAdmins < 1 {
		t.Fatalf("CRITICAL CONCURRENCY VIOLATION: system left with %d admins", remainingAdmins)
	}

	t.Logf("Admin concurrency test passed: %d demoted, %d rejected. Remaining admins in DB: %d",
		successCount, rejectedCount, remainingAdmins)
}
