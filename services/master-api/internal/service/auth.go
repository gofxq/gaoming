package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofxq/gaoming/services/master-api/internal/auth"
)

var (
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrAuthNotConfigured = errors.New("wechat login is not configured")
	ErrInvalidAuthState  = errors.New("invalid auth state")
)

type AuthSessionView struct {
	Authenticated bool      `json:"authenticated"`
	WeChatEnabled bool      `json:"wechat_enabled"`
	User          auth.User `json:"user"`
}

type WeChatLoginResponse struct {
	AuthURL string `json:"auth_url"`
}

type authState struct {
	ReturnTo   string `json:"return_to"`
	TenantCode string `json:"tenant_code"`
	IssuedAt   int64  `json:"issued_at"`
	Nonce      string `json:"nonce"`
}

func (s *Service) GetWeChatLoginURL(returnTo string, tenantCode string) (WeChatLoginResponse, error) {
	if s.weChatOAuth == nil || !s.weChatOAuth.IsEnabled() {
		return WeChatLoginResponse{}, ErrAuthNotConfigured
	}
	stateValue, err := s.signAuthState(authState{
		ReturnTo:   sanitizeReturnTo(returnTo, tenantCode),
		TenantCode: sanitizeTenantCode(tenantCode),
		IssuedAt:   s.clock.Now().UTC().Unix(),
		Nonce:      randomHex(12),
	})
	if err != nil {
		return WeChatLoginResponse{}, err
	}
	return WeChatLoginResponse{AuthURL: s.weChatOAuth.AuthURL(stateValue)}, nil
}

func (s *Service) HandleWeChatCallback(ctx context.Context, code string, stateValue string, ip string, userAgent string) (auth.Session, string, error) {
	if s.weChatOAuth == nil || !s.weChatOAuth.IsEnabled() {
		return auth.Session{}, "", ErrAuthNotConfigured
	}
	if strings.TrimSpace(code) == "" {
		return auth.Session{}, "", ErrUnauthorized
	}
	state, err := s.verifyAuthState(stateValue)
	if err != nil {
		return auth.Session{}, "", err
	}
	profile, err := s.weChatOAuth.Exchange(ctx, code)
	if err != nil {
		return auth.Session{}, "", err
	}
	user, err := s.authStore.CreateOrUpdateWeChatUser(ctx, state.TenantCode, profile, s.clock.Now().UTC())
	if err != nil {
		return auth.Session{}, "", err
	}

	token, tokenHash, err := generateSessionToken()
	if err != nil {
		return auth.Session{}, "", err
	}
	expiresAt := s.clock.Now().UTC().Add(s.sessionTTL())
	if err := s.authStore.CreateSession(ctx, user.ID, tokenHash, expiresAt, ip, userAgent, s.clock.Now().UTC()); err != nil {
		return auth.Session{}, "", err
	}
	return auth.Session{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user,
	}, sanitizeReturnTo(state.ReturnTo, user.TenantCode), nil
}

func (s *Service) GetSession(ctx context.Context, rawToken string) (AuthSessionView, error) {
	user, _, ok, err := s.ResolveSessionUser(ctx, rawToken)
	if err != nil {
		return AuthSessionView{}, err
	}
	if !ok {
		return AuthSessionView{
			Authenticated: false,
			WeChatEnabled: s.weChatOAuth != nil && s.weChatOAuth.IsEnabled(),
		}, nil
	}
	return AuthSessionView{
		Authenticated: true,
		WeChatEnabled: s.weChatOAuth != nil && s.weChatOAuth.IsEnabled(),
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

func (s *Service) signAuthState(state authState) (string, error) {
	if s.authConfig.SessionSecret == "" {
		return "", ErrAuthNotConfigured
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(s.authConfig.SessionSecret))
	mac.Write([]byte(encodedPayload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedPayload + "." + signature, nil
}

func (s *Service) verifyAuthState(value string) (authState, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return authState{}, ErrInvalidAuthState
	}
	mac := hmac.New(sha256.New, []byte(s.authConfig.SessionSecret))
	mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(got, expected) {
		return authState{}, ErrInvalidAuthState
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return authState{}, ErrInvalidAuthState
	}
	var state authState
	if err := json.Unmarshal(raw, &state); err != nil {
		return authState{}, ErrInvalidAuthState
	}
	if state.IssuedAt == 0 || s.clock.Now().UTC().Unix()-state.IssuedAt > int64((10*time.Minute).Seconds()) {
		return authState{}, ErrInvalidAuthState
	}
	state.TenantCode = sanitizeTenantCode(state.TenantCode)
	state.ReturnTo = sanitizeReturnTo(state.ReturnTo, state.TenantCode)
	return state, nil
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

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

func sanitizeTenantCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}
	return value
}

func sanitizeReturnTo(value string, tenantCode string) string {
	tenantCode = sanitizeTenantCode(tenantCode)
	value = strings.TrimSpace(value)
	if value == "" {
		return "/" + tenantCode
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || strings.HasPrefix(value, "//") {
		return "/" + tenantCode
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return value
}
