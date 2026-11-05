package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthService manages user registration, authentication, and JWT token operations.
type AuthService struct {
	userRepo repository.UserRepository
	auditSvc *AuditService
	cfg      *config.Config
}

// NewAuthService constructs an AuthService instance.
func NewAuthService(userRepo repository.UserRepository, auditSvc *AuditService, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		auditSvc: auditSvc,
		cfg:      cfg,
	}
}

// RegisterInput models the POST /api/v1/auth/register request payload.
type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// LoginInput models the POST /api/v1/auth/login request payload.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse models the successful login response with JWT token.
type LoginResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      UserResponse `json:"user"`
}

// UserResponse is a safe user representation that never includes password_hash.
type UserResponse struct {
	ID       string      `json:"id"`
	Email    string      `json:"email"`
	FullName string      `json:"full_name"`
	Role     domain.Role `json:"role"`
}

// Register creates a new user account with email/password validation and bcrypt hashing.
// The first registered user is automatically assigned "admin" role;
// all subsequent users receive "viewer" role.
// (Simple bootstrap approach for academic project.)
func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*UserResponse, error) {
	// Validate email format
	email := strings.TrimSpace(strings.ToLower(input.Email))
	if !domain.ValidateEmail(email) {
		return nil, &domain.ValidationError{Details: []string{"invalid email format"}}
	}

	// Validate password length
	if !domain.ValidatePassword(input.Password) {
		return nil, &domain.ValidationError{Details: []string{"password must be at least 8 characters"}}
	}

	// Hash password with bcrypt (cost 10)
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Determine role: first user is admin, rest are viewer
	role := domain.RoleViewer
	count, err := s.userRepo.Count(ctx)
	if err != nil {
		log.Printf("[WARN] Failed to count users for role bootstrap, defaulting to viewer: %v", err)
	} else if count == 0 {
		role = domain.RoleAdmin
	}

	user := domain.User{
		Email:        email,
		PasswordHash: string(hashBytes),
		FullName:     strings.TrimSpace(input.FullName),
		Role:         role,
	}

	saved, err := s.userRepo.Create(ctx, user)
	if err != nil {
		if errors.Is(err, domain.ErrEmailAlreadyExists) {
			return nil, domain.ErrEmailAlreadyExists
		}
		return nil, err
	}

	// Best-effort audit log (email in metadata, never password)
	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionAuthRegistered, "user", &saved.ID, map[string]any{
			"email": saved.Email,
			"role":  string(saved.Role),
		})
	}

	return &UserResponse{
		ID:       saved.ID,
		Email:    saved.Email,
		FullName: saved.FullName,
		Role:     saved.Role,
	}, nil
}

// Login authenticates a user by email/password and returns a JWT token on success.
// Returns ErrInvalidCredentials for both unknown email and wrong password
// to avoid leaking whether an email is registered.
// A dummy bcrypt compare is performed when the user is not found to reduce timing leaks.
func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginResponse, error) {
	email := strings.TrimSpace(strings.ToLower(input.Email))

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Dummy bcrypt compare to reduce timing side-channel leakage
			_ = bcrypt.CompareHashAndPassword(
				[]byte("$2a$10$dummyhashfortimingequalizationpurposes1234567890"),
				[]byte(input.Password),
			)

			// Best-effort audit: login failed
			if s.auditSvc != nil {
				s.auditSvc.Record(ctx, domain.ActionAuthLoginFailed, "user", nil, map[string]any{
					"email": email,
				})
			}
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	// Compare provided password with stored hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		// Best-effort audit: login failed
		if s.auditSvc != nil {
			s.auditSvc.Record(ctx, domain.ActionAuthLoginFailed, "user", &user.ID, map[string]any{
				"email": email,
			})
		}
		return nil, domain.ErrInvalidCredentials
	}

	// Generate JWT token
	token, expiresAt, err := s.generateToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Best-effort audit: login success
	if s.auditSvc != nil {
		authCtx := domain.WithUserContext(ctx, user.ID, user.Email, user.Role)
		s.auditSvc.Record(authCtx, domain.ActionAuthLogin, "user", &user.ID, map[string]any{
			"email": user.Email,
		})
	}

	return &LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: UserResponse{
			ID:       user.ID,
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role,
		},
	}, nil
}

// GetCurrentUser retrieves the user profile for the authenticated user ID from context.
func (s *AuthService) GetCurrentUser(ctx context.Context) (*UserResponse, error) {
	userID := domain.UserIDFromContext(ctx)
	if userID == "" {
		return nil, domain.ErrUnauthorized
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:       user.ID,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     user.Role,
	}, nil
}

// generateToken creates an HS256 JWT token with sub, email, role, exp, iat claims.
func (s *AuthService) generateToken(user *domain.User) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.cfg.JWTTTL)

	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"role":  string(user.Role),
		"iat":   now.Unix(),
		"exp":   expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

// ValidateToken parses and validates a JWT token string, returning claims if valid.
// Returns ErrUnauthorized for invalid, expired, or tampered tokens.
func (s *AuthService) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, domain.ErrUnauthorized
	}

	return claims, nil
}

// ListUsers retrieves all registered users without password hashes.
func (s *AuthService) ListUsers(ctx context.Context) ([]UserResponse, error) {
	users, err := s.userRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	resp := make([]UserResponse, len(users))
	for i, u := range users {
		resp[i] = UserResponse{
			ID:       u.ID,
			Email:    u.Email,
			FullName: u.FullName,
			Role:     u.Role,
		}
	}
	return resp, nil
}

// UpdateRoleInput models the payload for PATCH /api/v1/users/{id}/role.
type UpdateRoleInput struct {
	Role string `json:"role"`
}

// UpdateRole changes the role of a user with sole-admin demotion protection and audit logging.
func (s *AuthService) UpdateRole(ctx context.Context, targetUserID string, newRoleStr string) (*UserResponse, error) {
	if !uuidRegex.MatchString(targetUserID) {
		return nil, domain.ErrInvalidID
	}

	newRole := domain.Role(strings.TrimSpace(strings.ToLower(newRoleStr)))
	if !newRole.IsValid() {
		return nil, domain.ErrInvalidRole
	}

	targetUser, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	// Prevent demoting the only remaining admin in the system
	if targetUser.Role == domain.RoleAdmin && newRole != domain.RoleAdmin {
		adminCount, err := s.userRepo.CountAdmins(ctx)
		if err != nil {
			return nil, err
		}
		if adminCount <= 1 {
			return nil, domain.ErrCannotDemoteOnlyAdmin
		}
	}

	updated, err := s.userRepo.UpdateRole(ctx, targetUserID, newRole)
	if err != nil {
		return nil, err
	}

	// Best-effort audit log
	if s.auditSvc != nil {
		s.auditSvc.Record(ctx, domain.ActionUserRoleChanged, "user", &targetUserID, map[string]any{
			"old_role": string(targetUser.Role),
			"new_role": string(newRole),
		})
	}

	return &UserResponse{
		ID:       updated.ID,
		Email:    updated.Email,
		FullName: updated.FullName,
		Role:     updated.Role,
	}, nil
}
