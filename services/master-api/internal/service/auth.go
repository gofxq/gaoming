package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofxq/gaoming/services/master-api/internal/auth"
)

var (
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrAuthNotConfigured = errors.New("auth is not configured")
)

type AuthSessionView struct {
	Authenticated bool      `json:"authenticated"`
	User          auth.User `json:"user"`
}

func (s *Service) GetSession(ctx context.Context, rawToken string) (AuthSessionView, error) {
	user, _, ok, err := s.ResolveSessionUser(ctx, rawToken)
	if err != nil {
		return AuthSessionView{}, err
	}
	if !ok {
		return AuthSessionView{
			Authenticated: false,
		}, nil
	}
	return AuthSessionView{
		Authenticated: true,
		User:          user,
	}, nil
}

func (s *Service) ResolveSessionUser(ctx context.Context, rawToken string) (auth.User, auth.SessionRecord, bool, error) {
	if s.authStore == nil {
		return auth.User{}, auth.SessionRecord{}, false, nil
	}
	tokenHash := hashToken(rawToken)
	if tokenHash == "" {
		return auth.User{}, auth.SessionRecord{}, false, nil
	}
	return s.authStore.GetUserBySessionTokenHash(ctx, tokenHash, s.clock.Now().UTC())
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if s.authStore == nil {
		return nil
	}
	tokenHash := hashToken(rawToken)
	if tokenHash == "" {
		return nil
	}
	return s.authStore.DeleteSession(ctx, tokenHash)
}

func (s *Service) ListUsers(ctx context.Context, requester auth.User) ([]auth.User, error) {
	if requester.Role != auth.UserRoleAdmin {
		return nil, ErrForbidden
	}
	if s.authStore == nil {
		return nil, ErrAuthNotConfigured
	}
	return s.authStore.ListUsers(ctx, requester.TenantCode)
}

func (s *Service) UpdateUser(ctx context.Context, requester auth.User, userID int64, patch auth.UserUpdate) (auth.User, error) {
	if requester.Role != auth.UserRoleAdmin {
		return auth.User{}, ErrForbidden
	}
	if patch.Role != nil && *patch.Role != auth.UserRoleAdmin && *patch.Role != auth.UserRoleMember {
		return auth.User{}, fmt.Errorf("invalid role %q", *patch.Role)
	}
	if patch.Status != nil && *patch.Status != auth.UserStatusActive && *patch.Status != auth.UserStatusDisabled {
		return auth.User{}, fmt.Errorf("invalid status %q", *patch.Status)
	}
	if s.authStore == nil {
		return auth.User{}, ErrAuthNotConfigured
	}
	return s.authStore.UpdateUser(ctx, requester.TenantCode, userID, patch)
}

func (s *Service) sessionTTL() time.Duration {
	if s.authConfig.SessionTTL <= 0 {
		return 7 * 24 * time.Hour
	}
	return s.authConfig.SessionTTL
}

func (s *Service) SessionCookieName() string {
	return strings.TrimSpace(s.authConfig.SessionCookieName)
}

func generateSessionToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(rawToken string) string {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
