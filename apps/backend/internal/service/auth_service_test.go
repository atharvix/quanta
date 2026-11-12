package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type MockUserRepository struct {
	mu      sync.Mutex
	users   map[string]domain.User // keyed by email
	usersID map[string]domain.User // keyed by ID
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:   make(map[string]domain.User),
		usersID: make(map[string]domain.User),
	}
}

func (m *MockUserRepository) Create(ctx context.Context, user domain.User) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[user.Email]; exists {
		return nil, domain.ErrEmailAlreadyExists
	}

	if user.ID == "" {
		user.ID = fmt.Sprintf("123e4567-e89b-12d3-a456-%012d", len(m.users)+1)
	}
	user.CreatedAt = time.Now()
	m.users[user.Email] = user
	m.usersID[user.ID] = user

	return &user, nil
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[email]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return &user, nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.usersID[id]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return &user, nil
}

func (m *MockUserRepository) Count(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.users), nil
}

func (m *MockUserRepository) List(ctx context.Context) ([]domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var list []domain.User
	for _, u := range m.users {
		list = append(list, u)
	}
	return list, nil
}

func (m *MockUserRepository) UpdateRole(ctx context.Context, id string, role domain.Role) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.usersID[id]
	if !exists {
		return nil, domain.ErrNotFound
	}
	user.Role = role
	m.usersID[id] = user
	m.users[user.Email] = user
	return &user, nil
}

func (m *MockUserRepository) CountAdmins(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, u := range m.users {
		if u.Role == domain.RoleAdmin {
			count++
		}
	}
	return count, nil
}

func setupTestAuthService() (*AuthService, *MockUserRepository) {
	repo := NewMockUserRepository()
	cfg := &config.Config{
		JWTSecret: "test-secret-key-for-unit-testing",
		JWTTTL:    1 * time.Hour,
	}
	svc := NewAuthService(repo, nil, cfg)
	return svc, repo
}

func TestAuthService_Register(t *testing.T) {
	ctx := context.Background()

	t.Run("Validation: invalid email format", func(t *testing.T) {
		svc, _ := setupTestAuthService()
		_, err := svc.Register(ctx, RegisterInput{
			Email:    "notanemail",
			Password: "securepassword123",
			FullName: "Test User",
		})
		if err == nil {
			t.Fatal("expected validation error for invalid email, got nil")
		}
		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected *domain.ValidationError, got %T", err)
		}
	})

	t.Run("Validation: password shorter than 8 chars", func(t *testing.T) {
		svc, _ := setupTestAuthService()
		_, err := svc.Register(ctx, RegisterInput{
			Email:    "test@example.com",
			Password: "short",
			FullName: "Test User",
		})
		if err == nil {
			t.Fatal("expected validation error for short password, got nil")
		}
		var valErr *domain.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatalf("expected *domain.ValidationError, got %T", err)
		}
	})

	t.Run("Bootstrap roles: first user is admin, second user is viewer", func(t *testing.T) {
		svc, _ := setupTestAuthService()

		// First user registration
		user1, err := svc.Register(ctx, RegisterInput{
			Email:    "first@quanta.local",
			Password: "password123",
			FullName: "First Admin",
		})
		if err != nil {
			t.Fatalf("unexpected error registering user 1: %v", err)
		}
		if user1.Role != domain.RoleAdmin {
			t.Errorf("expected first user role 'admin', got '%s'", user1.Role)
		}

		// Second user registration
		user2, err := svc.Register(ctx, RegisterInput{
			Email:    "second@quanta.local",
			Password: "password123",
			FullName: "Second Viewer",
		})
		if err != nil {
			t.Fatalf("unexpected error registering user 2: %v", err)
		}
		if user2.Role != domain.RoleViewer {
			t.Errorf("expected second user role 'viewer', got '%s'", user2.Role)
		}
	})

	t.Run("Duplicate email returns ErrEmailAlreadyExists", func(t *testing.T) {
		svc, _ := setupTestAuthService()

		_, err := svc.Register(ctx, RegisterInput{
			Email:    "dupe@quanta.local",
			Password: "password123",
			FullName: "Original User",
		})
		if err != nil {
			t.Fatalf("unexpected error registering initial user: %v", err)
		}

		_, err = svc.Register(ctx, RegisterInput{
			Email:    "dupe@quanta.local",
			Password: "newpassword123",
			FullName: "Duplicate User",
		})
		if !errors.Is(err, domain.ErrEmailAlreadyExists) {
			t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
		}
	})
}

func TestAuthService_Login(t *testing.T) {
	ctx := context.Background()

	t.Run("Successful login returns valid token and user profile", func(t *testing.T) {
		svc, _ := setupTestAuthService()

		_, err := svc.Register(ctx, RegisterInput{
			Email:    "analyst@quanta.local",
			Password: "correctpassword",
			FullName: "Test Analyst",
		})
		if err != nil {
			t.Fatalf("unexpected register error: %v", err)
		}

		resp, err := svc.Login(ctx, LoginInput{
			Email:    "analyst@quanta.local",
			Password: "correctpassword",
		})
		if err != nil {
			t.Fatalf("unexpected login error: %v", err)
		}

		if resp.Token == "" {
			t.Error("expected non-empty token")
		}
		if resp.User.Email != "analyst@quanta.local" {
			t.Errorf("expected user email analyst@quanta.local, got %s", resp.User.Email)
		}
		if resp.ExpiresAt.Before(time.Now()) {
			t.Error("expected expires_at to be in the future")
		}
	})

	t.Run("Wrong password returns ErrInvalidCredentials", func(t *testing.T) {
		svc, _ := setupTestAuthService()

		_, err := svc.Register(ctx, RegisterInput{
			Email:    "analyst2@quanta.local",
			Password: "correctpassword",
			FullName: "Test Analyst 2",
		})
		if err != nil {
			t.Fatalf("unexpected register error: %v", err)
		}

		_, err = svc.Login(ctx, LoginInput{
			Email:    "analyst2@quanta.local",
			Password: "WRONGpassword",
		})
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("Unknown email returns same ErrInvalidCredentials", func(t *testing.T) {
		svc, _ := setupTestAuthService()

		_, err := svc.Login(ctx, LoginInput{
			Email:    "unknown@quanta.local",
			Password: "somelongpassword",
		})
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestAuthService_JWT(t *testing.T) {
	cfg := &config.Config{
		JWTSecret: "test-secret-key-1234567890",
		JWTTTL:    10 * time.Minute,
	}
	repo := NewMockUserRepository()
	svc := NewAuthService(repo, nil, cfg)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass1234"), bcrypt.DefaultCost)
	user := &domain.User{
		ID:           "user-jwt-1",
		Email:        "jwt@quanta.local",
		PasswordHash: string(hash),
		FullName:     "JWT Tester",
		Role:         domain.RoleAnalyst,
	}

	t.Run("Valid token verifies claims correctly", func(t *testing.T) {
		token, _, err := svc.generateToken(user)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		claims, err := svc.ValidateToken(token)
		if err != nil {
			t.Fatalf("expected valid token, got error: %v", err)
		}

		if claims["sub"] != "user-jwt-1" {
			t.Errorf("expected sub 'user-jwt-1', got '%v'", claims["sub"])
		}
		if claims["email"] != "jwt@quanta.local" {
			t.Errorf("expected email 'jwt@quanta.local', got '%v'", claims["email"])
		}
		if claims["role"] != "analyst" {
			t.Errorf("expected role 'analyst', got '%v'", claims["role"])
		}
	})

	t.Run("Expired token returns ErrUnauthorized", func(t *testing.T) {
		// Generate expired token
		expiredClaims := jwt.MapClaims{
			"sub":   user.ID,
			"email": user.Email,
			"role":  string(user.Role),
			"iat":   time.Now().Add(-2 * time.Hour).Unix(),
			"exp":   time.Now().Add(-1 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
		signed, err := token.SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			t.Fatalf("failed to sign expired token: %v", err)
		}

		_, err = svc.ValidateToken(signed)
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized for expired token, got %v", err)
		}
	})

	t.Run("Tampered signature returns ErrUnauthorized", func(t *testing.T) {
		token, _, err := svc.generateToken(user)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		// Tamper token payload by replacing characters
		parts := strings.Split(token, ".")
		if len(parts) == 3 {
			tampered := parts[0] + "." + parts[1] + "extra." + parts[2]
			_, err = svc.ValidateToken(tampered)
			if !errors.Is(err, domain.ErrUnauthorized) {
				t.Fatalf("expected ErrUnauthorized for tampered token, got %v", err)
			}
		}
	})

	t.Run("Wrong signing secret returns ErrUnauthorized", func(t *testing.T) {
		wrongClaims := jwt.MapClaims{
			"sub":   user.ID,
			"email": user.Email,
			"role":  string(user.Role),
			"iat":   time.Now().Unix(),
			"exp":   time.Now().Add(time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, wrongClaims)
		signed, _ := token.SignedString([]byte("completely-different-secret"))

		_, err := svc.ValidateToken(signed)
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized for wrong signature secret, got %v", err)
		}
	})
}

func TestAuthService_ListUsers(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestAuthService()

	_, _ = svc.Register(ctx, RegisterInput{
		Email:    "u1@quanta.local",
		Password: "password123",
		FullName: "User One",
	})
	_, _ = svc.Register(ctx, RegisterInput{
		Email:    "u2@quanta.local",
		Password: "password123",
		FullName: "User Two",
	})

	users, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatalf("unexpected error listing users: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestAuthService_UpdateRole(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestAuthService()

	// Register admin (first)
	admin, err := svc.Register(ctx, RegisterInput{
		Email:    "admin@quanta.local",
		Password: "password123",
		FullName: "Only Admin",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Register viewer (second)
	viewer, err := svc.Register(ctx, RegisterInput{
		Email:    "viewer@quanta.local",
		Password: "password123",
		FullName: "Viewer User",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	t.Run("Invalid UUID returns ErrInvalidID", func(t *testing.T) {
		_, err := svc.UpdateRole(ctx, "invalid-uuid", "analyst")
		if !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("expected ErrInvalidID, got %v", err)
		}
	})

	t.Run("Invalid role returns ErrInvalidRole", func(t *testing.T) {
		_, err := svc.UpdateRole(ctx, viewer.ID, "superman")
		if !errors.Is(err, domain.ErrInvalidRole) {
			t.Fatalf("expected ErrInvalidRole, got %v", err)
		}
	})

	t.Run("Cannot demote the only admin returns ErrCannotDemoteOnlyAdmin", func(t *testing.T) {
		_, err := svc.UpdateRole(ctx, admin.ID, "analyst")
		if !errors.Is(err, domain.ErrCannotDemoteOnlyAdmin) {
			t.Fatalf("expected ErrCannotDemoteOnlyAdmin, got %v", err)
		}
	})

	t.Run("Promoting viewer to analyst succeeds", func(t *testing.T) {
		updated, err := svc.UpdateRole(ctx, viewer.ID, "analyst")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Role != domain.RoleAnalyst {
			t.Errorf("expected role 'analyst', got '%s'", updated.Role)
		}
	})

	t.Run("Promoting second user to admin allows demoting first admin", func(t *testing.T) {
		// Promote second user to admin
		_, err := svc.UpdateRole(ctx, viewer.ID, "admin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Now there are 2 admins; demoting the first admin should succeed
		updated, err := svc.UpdateRole(ctx, admin.ID, "viewer")
		if err != nil {
			t.Fatalf("unexpected error demoting admin when 2 exist: %v", err)
		}
		if updated.Role != domain.RoleViewer {
			t.Errorf("expected role 'viewer', got '%s'", updated.Role)
		}
	})
}

type mockAuditRepoForAuth struct {
	mu   sync.Mutex
	logs []domain.AuditLog
}

func (m *mockAuditRepoForAuth) Create(ctx context.Context, l domain.AuditLog) (*domain.AuditLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, l)
	return &l, nil
}

func (m *mockAuditRepoForAuth) List(ctx context.Context, limit, offset int, entityType, action string) ([]domain.AuditLog, error) {
	return nil, nil
}

func TestAuthService_AuditActions(t *testing.T) {
	ctx := context.Background()
	repo := NewMockUserRepository()
	auditRepo := &mockAuditRepoForAuth{}
	auditSvc := NewAuditService(auditRepo)
	cfg := &config.Config{
		JWTSecret: "test-secret-actions",
		JWTTTL:    time.Hour,
	}
	svc := NewAuthService(repo, auditSvc, cfg)

	// 1. Register triggers auth.registered
	user, err := svc.Register(ctx, RegisterInput{
		Email:    "audit_tester@quanta.local",
		Password: "password123",
		FullName: "Audit Tester",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	auditRepo.mu.Lock()
	if len(auditRepo.logs) != 1 || auditRepo.logs[0].Action != domain.ActionAuthRegistered {
		t.Errorf("expected ActionAuthRegistered, got %v", auditRepo.logs)
	}
	auditRepo.mu.Unlock()

	// 2. Login failure triggers auth.login_failed with actor anonymous
	_, _ = svc.Login(ctx, LoginInput{
		Email:    "audit_tester@quanta.local",
		Password: "wrongpassword",
	})

	auditRepo.mu.Lock()
	lastLog := auditRepo.logs[len(auditRepo.logs)-1]
	if lastLog.Action != domain.ActionAuthLoginFailed {
		t.Errorf("expected ActionAuthLoginFailed, got %s", lastLog.Action)
	}
	if lastLog.Actor != "anonymous" {
		t.Errorf("expected actor 'anonymous', got %s", lastLog.Actor)
	}
	if strings.Contains(string(lastLog.Metadata), "wrongpassword") {
		t.Errorf("password should never be present in audit metadata")
	}
	auditRepo.mu.Unlock()

	// 3. Login success triggers auth.login with user email as actor
	_, err = svc.Login(ctx, LoginInput{
		Email:    "audit_tester@quanta.local",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	auditRepo.mu.Lock()
	lastLog = auditRepo.logs[len(auditRepo.logs)-1]
	if lastLog.Action != domain.ActionAuthLogin {
		t.Errorf("expected ActionAuthLogin, got %s", lastLog.Action)
	}
	if lastLog.Actor != "audit_tester@quanta.local" {
		t.Errorf("expected actor 'audit_tester@quanta.local', got %s", lastLog.Actor)
	}
	auditRepo.mu.Unlock()

	// 4. UpdateRole triggers user.role_changed
	authCtx := domain.WithUserContext(ctx, "admin-1", "admin@quanta.local", domain.RoleAdmin)
	user2, _ := svc.Register(ctx, RegisterInput{Email: "user2@quanta.local", Password: "password123"})
	_, err = svc.UpdateRole(authCtx, user2.ID, "analyst")
	if err != nil {
		t.Fatalf("update role failed: %v", err)
	}

	auditRepo.mu.Lock()
	lastLog = auditRepo.logs[len(auditRepo.logs)-1]
	if lastLog.Action != domain.ActionUserRoleChanged {
		t.Errorf("expected ActionUserRoleChanged, got %s", lastLog.Action)
	}
	if lastLog.Actor != "admin@quanta.local" {
		t.Errorf("expected actor 'admin@quanta.local', got %s", lastLog.Actor)
	}
	if !strings.Contains(string(lastLog.Metadata), "old_role") || !strings.Contains(string(lastLog.Metadata), "new_role") {
		t.Errorf("expected old_role and new_role in metadata, got %s", string(lastLog.Metadata))
	}
	auditRepo.mu.Unlock()

	_ = user
}
