package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gofxq/gaoming/server/internal/master/auth"
)

type LoginRequest struct {
	TenantCode string `json:"tenant_code"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type SetupRequest struct {
	SetupToken  string `json:"setup_token"`
	TenantCode  string `json:"tenant_code"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{2,63}$`)
var tenantPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func localAccount(req SetupRequest) (auth.LocalAccount, error) {
	username := strings.ToLower(strings.TrimSpace(req.Username))
	tenant := strings.TrimSpace(req.TenantCode)
	displayName := strings.TrimSpace(req.DisplayName)
	if !usernamePattern.MatchString(username) || !tenantPattern.MatchString(tenant) || len(displayName) == 0 || len(displayName) > 128 {
		return auth.LocalAccount{}, fmt.Errorf("invalid tenant, username or display name")
	}
	hash, err := auth.HashPassword(req.Password)
	return auth.LocalAccount{TenantCode: tenant, Username: username, DisplayName: displayName, PasswordHash: hash}, err
}

func (s *Service) SetupAvailable(ctx context.Context) (bool, error) {
	if s.authStore == nil || len(s.authConfig.SetupToken) < 32 {
		return false, nil
	}
	return s.authStore.SetupAvailable(ctx)
}

func (s *Service) Setup(ctx context.Context, req SetupRequest) (auth.Session, error) {
	if s.authStore == nil || len(s.authConfig.SetupToken) < 32 {
		return auth.Session{}, ErrAuthNotConfigured
	}
	actual, expected := sha256.Sum256([]byte(req.SetupToken)), sha256.Sum256([]byte(s.authConfig.SetupToken))
	if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
		return auth.Session{}, ErrUnauthorized
	}
	account, err := localAccount(req)
	if err != nil {
		return auth.Session{}, err
	}
	user, err := s.authStore.BootstrapAdmin(ctx, account, s.clock.Now().UTC())
	if errors.Is(err, auth.ErrSetupUnavailable) {
		return auth.Session{}, ErrForbidden
	}
	if err != nil {
		return auth.Session{}, fmt.Errorf("administrator setup failed")
	}
	return s.createLoginSession(ctx, user)
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (auth.Session, error) {
	if s.authStore == nil {
		return auth.Session{}, ErrAuthNotConfigured
	}
	// Initialize dummy work on every first login, independent of account presence.
	hash := auth.DummyPasswordHash()
	user, stored, found, err := s.authStore.LocalCredentials(ctx, strings.TrimSpace(req.TenantCode), strings.ToLower(strings.TrimSpace(req.Username)))
	if err != nil {
		return auth.Session{}, fmt.Errorf("login unavailable")
	}
	if found {
		hash = stored
	}
	verified := auth.VerifyPassword(hash, req.Password)
	if !found || !verified || user.Status != auth.UserStatusActive {
		return auth.Session{}, ErrUnauthorized
	}
	return s.createLoginSession(ctx, user)
}

func (s *Service) createLoginSession(ctx context.Context, user auth.User) (auth.Session, error) {
	value, hash, err := generateSessionToken()
	if err != nil {
		return auth.Session{}, fmt.Errorf("session creation failed")
	}
	now := s.clock.Now().UTC()
	expires := now.Add(s.sessionTTL())
	if err := s.authStore.CreateSession(ctx, user.ID, hash, expires, "", "", now); err != nil {
		return auth.Session{}, fmt.Errorf("session creation failed")
	}
	return auth.Session{Token: value, ExpiresAt: expires, User: user}, nil
}

func (s *Service) CookieSecure() bool       { return s.authConfig.CookieSecure }
func (s *Service) AllowedOrigins() []string { return s.authConfig.AllowedOrigins }
