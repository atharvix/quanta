package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid or expired access token")
)

// TokenClaims represents validated JWT claims for an authenticated user.
type TokenClaims struct {
	Subject   uuid.UUID `json:"-"`
	SubRaw    string    `json:"sub"`
	Email     string    `json:"email"`
	Issuer    string    `json:"iss"`
	Audience  string    `json:"aud"`
	IssuedAt  int64     `json:"iat"`
	ExpiresAt int64     `json:"exp"`
	TokenID   string    `json:"jti"`
}

// TokenService signs and validates HS256 JWT access tokens.
type TokenService struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	nowFn    func() time.Time
}

// NewTokenService creates a TokenService with explicit secret, issuer, audience, and TTL.
func NewTokenService(secret, issuer, audience string, ttl time.Duration) (*TokenService, error) {
	if len(strings.TrimSpace(secret)) < 32 {
		return nil, errors.New("JWT secret must be at least 32 characters")
	}
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" {
		return nil, errors.New("JWT issuer and audience must not be empty")
	}
	if ttl <= 0 {
		return nil, errors.New("JWT TTL must be positive")
	}
	return &TokenService{
		secret:   []byte(secret),
		issuer:   issuer,
		audience: audience,
		ttl:      ttl,
		nowFn:    time.Now,
	}, nil
}

// IssueAccessToken signs a short-lived HS256 JWT for userID and email.
func (s *TokenService) IssueAccessToken(userID uuid.UUID, email string) (string, time.Time, error) {
	now := s.nowFn().UTC()
	exp := now.Add(s.ttl)

	headerJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", time.Time{}, err
	}

	claims := TokenClaims{
		SubRaw:    userID.String(),
		Email:     email,
		Issuer:    s.issuer,
		Audience:  s.audience,
		IssuedAt:  now.Unix(),
		ExpiresAt: exp.Unix(),
		TokenID:   uuid.NewString(),
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}

	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)
	sig := s.sign(unsigned)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig), exp, nil
}

// VerifyAccessToken validates signature, algorithm, issuer, audience, and expiry.
func (s *TokenService) VerifyAccessToken(token string) (TokenClaims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return TokenClaims{}, ErrInvalidToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Alg != "HS256" {
		return TokenClaims{}, ErrInvalidToken
	}

	unsigned := parts[0] + "." + parts[1]
	providedSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	expectedSig := s.sign(unsigned)
	if !hmac.Equal(providedSig, expectedSig) {
		return TokenClaims{}, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	var claims TokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return TokenClaims{}, ErrInvalidToken
	}

	nowUnix := s.nowFn().UTC().Unix()
	if claims.ExpiresAt <= nowUnix {
		return TokenClaims{}, fmt.Errorf("%w: token expired", ErrInvalidToken)
	}
	if claims.Issuer != s.issuer || claims.Audience != s.audience {
		return TokenClaims{}, fmt.Errorf("%w: issuer or audience mismatch", ErrInvalidToken)
	}
	subID, err := uuid.Parse(claims.SubRaw)
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	claims.Subject = subID
	return claims, nil
}

func (s *TokenService) sign(unsigned string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(unsigned))
	return mac.Sum(nil)
}
