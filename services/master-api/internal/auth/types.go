package auth

import (
	"context"
	"time"
)

type UserRole string

const (
	UserRoleAdmin  UserRole = "admin"
	UserRoleMember UserRole = "member"
)

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

type User struct {
	ID             int64      `json:"id"`
	TenantCode     string     `json:"tenant_code"`
	DisplayName    string     `json:"display_name"`
	AvatarURL      string     `json:"avatar_url,omitempty"`
	Role           UserRole   `json:"role"`
	Status         UserStatus `json:"status"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Provider       string     `json:"provider,omitempty"`
	ProviderUserID string     `json:"provider_user_id,omitempty"`
}

type UserUpdate struct {
	DisplayName *string
	Role        *UserRole
	Status      *UserStatus
}

type Session struct {
	Token     string
	ExpiresAt time.Time
	User      User
}

type SessionRecord struct {
	ID        int64
	UserID    int64
	ExpiresAt time.Time
}

type WeChatProfile struct {
	OpenID      string `json:"openid"`
	UnionID     string `json:"unionid,omitempty"`
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"headimgurl,omitempty"`
	Country     string `json:"country,omitempty"`
	Province    string `json:"province,omitempty"`
	City        string `json:"city,omitempty"`
	Language    string `json:"language,omitempty"`
	Sex         int    `json:"sex,omitempty"`
	AccessToken string `json:"-"`
}

type Store interface {
	CreateOrUpdateWeChatUser(ctx context.Context, tenantCode string, profile WeChatProfile, now time.Time) (User, error)
	CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, ip string, userAgent string, now time.Time) error
	GetUserBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (User, SessionRecord, bool, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	ListUsers(ctx context.Context, tenantCode string) ([]User, error)
	UpdateUser(ctx context.Context, tenantCode string, userID int64, patch UserUpdate) (User, error)
}
