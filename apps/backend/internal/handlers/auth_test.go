package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type mockUserRepoForHandler struct {
	users   map[string]domain.User
	usersID map[string]domain.User
}

func newMockUserRepoForHandler() *mockUserRepoForHandler {
	return &mockUserRepoForHandler{
		users:   make(map[string]domain.User),
		usersID: make(map[string]domain.User),
	}
}

func (m *mockUserRepoForHandler) Create(ctx context.Context, u domain.User) (*domain.User, error) {
	if _, exists := m.users[u.Email]; exists {
		return nil, domain.ErrEmailAlreadyExists
	}
	if u.ID == "" {
		u.ID = fmt.Sprintf("123e4567-e89b-12d3-a456-%012d", len(m.users)+1)
	}
	u.CreatedAt = time.Now()
	m.users[u.Email] = u
	m.usersID[u.ID] = u
	return &u, nil
}

func (m *mockUserRepoForHandler) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, exists := m.users[email]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return &u, nil
}

func (m *mockUserRepoForHandler) GetByID(ctx context.Context, id string) (*domain.User, error) {
	u, exists := m.usersID[id]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return &u, nil
}

func (m *mockUserRepoForHandler) Count(ctx context.Context) (int, error) {
	return len(m.users), nil
}

func (m *mockUserRepoForHandler) List(ctx context.Context) ([]domain.User, error) {
	var list []domain.User
	for _, u := range m.users {
		list = append(list, u)
	}
	return list, nil
}

func (m *mockUserRepoForHandler) UpdateRole(ctx context.Context, id string, role domain.Role) (*domain.User, error) {
	u, exists := m.usersID[id]
	if !exists {
		return nil, domain.ErrNotFound
	}
	u.Role = role
	m.usersID[id] = u
	m.users[u.Email] = u
	return &u, nil
}

func (m *mockUserRepoForHandler) CountAdmins(ctx context.Context) (int, error) {
	count := 0
	for _, u := range m.users {
		if u.Role == domain.RoleAdmin {
			count++
		}
	}
	return count, nil
}

func setupTestAuthRouter() (*chi.Mux, *service.AuthService) {
	repo := newMockUserRepoForHandler()
	cfg := &config.Config{
		JWTSecret: "auth-handler-test-secret-1234",
		JWTTTL:    time.Hour,
	}
	svc := service.NewAuthService(repo, nil, cfg)
	h := NewAuthHandler(svc)

	r := chi.NewRouter()
	r.Route("/api/v1/auth", func(auth chi.Router) {
		auth.Post("/register", h.Register)
		auth.Post("/login", h.Login)
		auth.Get("/me", h.Me)
	})
	r.Route("/api/v1/users", func(users chi.Router) {
		users.Get("/", h.ListUsers)
		users.Patch("/{id}/role", h.UpdateRole)
	})

	return r, svc
}

func TestAuthHandler_Register(t *testing.T) {
	r, _ := setupTestAuthRouter()

	t.Run("Success 201 Created", func(t *testing.T) {
		payload := `{"email":"admin@quanta.local","password":"securepassword123","full_name":"Admin User"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["email"] != "admin@quanta.local" {
			t.Errorf("expected email admin@quanta.local, got %v", resp["email"])
		}
		if resp["role"] != "admin" {
			t.Errorf("expected role admin for first user, got %v", resp["role"])
		}
		if _, hasHash := resp["password_hash"]; hasHash {
			t.Errorf("password_hash should never be returned in register response")
		}
	})

	t.Run("Duplicate email returns 409 Conflict", func(t *testing.T) {
		payload := `{"email":"admin@quanta.local","password":"anotherpassword123","full_name":"Another User"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Validation failure returns 400 Bad Request", func(t *testing.T) {
		payload := `{"email":"invalid-email","password":"short","full_name":"Test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestAuthHandler_Login(t *testing.T) {
	r, _ := setupTestAuthRouter()

	// Pre-register user
	regPayload := `{"email":"loginuser@quanta.local","password":"correctpassword123","full_name":"Login Tester"}`
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(regPayload))
	regRec := httptest.NewRecorder()
	r.ServeHTTP(regRec, regReq)

	t.Run("Successful login returns 200 with token and user", func(t *testing.T) {
		payload := `{"email":"loginuser@quanta.local","password":"correctpassword123"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["token"] == nil || resp["token"] == "" {
			t.Errorf("expected non-empty token")
		}
		if resp["expires_at"] == nil {
			t.Errorf("expected non-nil expires_at")
		}
		userObj, ok := resp["user"].(map[string]any)
		if !ok || userObj["email"] != "loginuser@quanta.local" {
			t.Errorf("expected user email in response, got %v", userObj)
		}
	})

	t.Run("Wrong password returns 401 with generic error", func(t *testing.T) {
		payload := `{"email":"loginuser@quanta.local","password":"WRONGpassword"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "invalid credentials") {
			t.Errorf("expected 'invalid credentials' in response body, got %s", rec.Body.String())
		}
	})

	t.Run("Unknown email returns 401 with identical generic error", func(t *testing.T) {
		payload := `{"email":"nobody@quanta.local","password":"wrongpassword"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "invalid credentials") {
			t.Errorf("expected 'invalid credentials' in response body, got %s", rec.Body.String())
		}
	})
}

func TestAuthHandler_Me(t *testing.T) {
	r, svc := setupTestAuthRouter()

	regUser, _ := svc.Register(context.Background(), service.RegisterInput{
		Email:    "profile@quanta.local",
		Password: "password123",
		FullName: "Profile Tester",
	})

	t.Run("Unauthenticated request returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Authenticated context returns 200 with profile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		authCtx := domain.WithUserContext(req.Context(), regUser.ID, regUser.Email, regUser.Role)
		req = req.WithContext(authCtx)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var user domain.User
		_ = json.Unmarshal(rec.Body.Bytes(), &user)
		if user.Email != "profile@quanta.local" {
			t.Errorf("expected profile@quanta.local, got %s", user.Email)
		}
	})
}

func TestAuthHandler_ListUsers(t *testing.T) {
	r, svc := setupTestAuthRouter()

	_, _ = svc.Register(context.Background(), service.RegisterInput{
		Email:    "list1@quanta.local",
		Password: "password123",
		FullName: "List User 1",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var users []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(users) == 0 {
		t.Fatalf("expected non-empty user list")
	}

	for _, u := range users {
		if _, hasHash := u["password_hash"]; hasHash {
			t.Errorf("password_hash must never be in users response")
		}
	}
}

func TestAuthHandler_UpdateRole(t *testing.T) {
	r, svc := setupTestAuthRouter()

	admin, _ := svc.Register(context.Background(), service.RegisterInput{
		Email:    "admin_role@quanta.local",
		Password: "password123",
		FullName: "Sole Admin",
	})
	viewer, _ := svc.Register(context.Background(), service.RegisterInput{
		Email:    "viewer_role@quanta.local",
		Password: "password123",
		FullName: "Viewer User",
	})

	t.Run("Invalid role returns 400 Bad Request", func(t *testing.T) {
		payload := `{"role":"invalid_role_name"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+viewer.ID+"/role", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Invalid UUID format returns 400 Bad Request", func(t *testing.T) {
		payload := `{"role":"analyst"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/not-a-valid-uuid/role", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Cannot demote sole admin returns 400", func(t *testing.T) {
		payload := `{"role":"viewer"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+admin.ID+"/role", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for sole admin demotion, got %d. Body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Successful role update returns 200 OK", func(t *testing.T) {
		payload := `{"role":"analyst"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+viewer.ID+"/role", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var updated map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &updated)
		if updated["role"] != "analyst" {
			t.Errorf("expected role 'analyst', got %v", updated["role"])
		}
	})
}
