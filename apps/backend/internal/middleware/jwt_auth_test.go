package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/golang-jwt/jwt/v5"
)

type mockUserRepoForMiddleware struct{}

func (m *mockUserRepoForMiddleware) Create(ctx context.Context, u domain.User) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForMiddleware) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForMiddleware) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForMiddleware) Count(ctx context.Context) (int, error) {
	return 0, nil
}
func (m *mockUserRepoForMiddleware) List(ctx context.Context) ([]domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForMiddleware) UpdateRole(ctx context.Context, id string, role domain.Role) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForMiddleware) CountAdmins(ctx context.Context) (int, error) {
	return 0, nil
}

func setupTestAuthMiddleware() (*service.AuthService, func(http.Handler) http.Handler, string) {
	jwtSecret := "jwt-auth-middleware-test-secret"
	cfg := &config.Config{
		JWTSecret: jwtSecret,
		JWTTTL:    time.Hour,
	}
	authSvc := service.NewAuthService(&mockUserRepoForMiddleware{}, nil, cfg)
	mw := JWTAuth(authSvc)
	return authSvc, mw, jwtSecret
}

func TestJWTAuthMiddleware(t *testing.T) {
	authSvc, mw, secret := setupTestAuthMiddleware()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := domain.UserIDFromContext(r.Context())
		email := domain.EmailFromContext(r.Context())
		role := domain.RoleFromContext(r.Context())

		if userID == "" || email == "" || role == "" {
			http.Error(w, "missing context values", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok:" + email))
	})

	protectedHandler := mw(nextHandler)

	t.Run("Missing Authorization header returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Non-Bearer format returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Basic somecredentials")
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Tampered token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid.jwt.token")
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Expired token returns 401", func(t *testing.T) {
		expiredClaims := jwt.MapClaims{
			"sub":   "u-123",
			"email": "user@quanta.local",
			"role":  "viewer",
			"iat":   time.Now().Add(-2 * time.Hour).Unix(),
			"exp":   time.Now().Add(-1 * time.Hour).Unix(),
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
		signed, _ := tok.SignedString([]byte(secret))

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+signed)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for expired token, got %d", rec.Code)
		}
	})

	t.Run("Valid token sets context and proceeds to next handler", func(t *testing.T) {
		validClaims := jwt.MapClaims{
			"sub":   "user-999",
			"email": "valid@quanta.local",
			"role":  "analyst",
			"iat":   time.Now().Unix(),
			"exp":   time.Now().Add(time.Hour).Unix(),
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims)
		signed, _ := tok.SignedString([]byte(secret))

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+signed)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "ok:valid@quanta.local" {
			t.Errorf("expected body 'ok:valid@quanta.local', got '%s'", rec.Body.String())
		}
	})

	_ = authSvc
}
