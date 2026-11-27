package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/auth"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// LoginResult contains the signed access token and safe user metadata (never password hash).
type LoginResult struct {
	AccessToken string
	TokenType   string
	ExpiresAt   time.Time
	User        domain.User
	Memberships []domain.OrganizationMembership
}

// AuthService handles credential verification, JWT issuance, and user membership lookup.
type AuthService struct {
	userRepo  repository.UserRepository
	auditRepo repository.AuditRepository
	tokens    *auth.TokenService
}

// NewAuthService constructs an AuthService.
func NewAuthService(
	userRepo repository.UserRepository,
	auditRepo repository.AuditRepository,
	tokens *auth.TokenService,
) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		auditRepo: auditRepo,
		tokens:    tokens,
	}
}

// Login authenticates email+password and returns a signed JWT access token.
// Never logs plaintext passwords or issued JWTs.
func (s *AuthService) Login(ctx context.Context, email, password, correlationID string) (LoginResult, error) {
	trimmedEmail := strings.TrimSpace(email)
	user, err := s.userRepo.GetUserByEmail(ctx, trimmedEmail)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			s.recordAuthAudit(ctx, nil, "AUTH_LOGIN_FAILED", trimmedEmail, "user_not_found", correlationID)
			return LoginResult{}, domain.ErrInvalidCredentials
		}
		return LoginResult{}, err
	}

	if !user.IsActive {
		s.recordAuthAudit(ctx, &user.ID, "AUTH_LOGIN_FAILED", user.Email, "user_inactive", correlationID)
		return LoginResult{}, domain.ErrInactiveUser
	}

	if err := auth.VerifyPassword(user.PasswordHash, password); err != nil {
		s.recordAuthAudit(ctx, &user.ID, "AUTH_LOGIN_FAILED", user.Email, "bad_password", correlationID)
		return LoginResult{}, domain.ErrInvalidCredentials
	}

	token, expiresAt, err := s.tokens.IssueAccessToken(user.ID, user.Email)
	if err != nil {
		return LoginResult{}, err
	}

	memberships, err := s.userRepo.ListMembershipsByUser(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}

	s.recordAuthAudit(ctx, &user.ID, "AUTH_LOGIN_SUCCESS", user.Email, "ok", correlationID)
	user.PasswordHash = ""
	return LoginResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		User:        user,
		Memberships: memberships,
	}, nil
}

// Me returns the authenticated user's profile (without password hash) and all organization memberships.
func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (domain.User, []domain.OrganizationMembership, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, nil, err
	}
	user.PasswordHash = ""

	memberships, err := s.userRepo.ListMembershipsByUser(ctx, userID)
	if err != nil {
		return domain.User{}, nil, err
	}
	return user, memberships, nil
}

func (s *AuthService) recordAuthAudit(ctx context.Context, actorID *uuid.UUID, action, email, outcome, correlationID string) {
	if s.auditRepo == nil {
		return
	}
	meta, _ := json.Marshal(map[string]any{
		"email":   email,
		"outcome": outcome,
	})
	_ = s.auditRepo.AppendAuditEvent(ctx, domain.AuditEvent{
		ID:            uuid.New(),
		ActorID:       actorID,
		ActorRole:     "USER",
		Action:        action,
		EntityType:    "user",
		EntityID:      actorID,
		CorrelationID: correlationID,
		Metadata:      meta,
	})
}
