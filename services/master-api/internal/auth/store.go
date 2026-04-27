package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const weChatProvider = "wechat"

type GormStore struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

func (s *GormStore) CreateOrUpdateWeChatUser(ctx context.Context, tenantCode string, profile WeChatProfile, now time.Time) (User, error) {
	if tenantCode == "" {
		tenantCode = "default"
	}
	if profile.OpenID == "" {
		return User{}, errors.New("missing wechat openid")
	}

	var result User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant, err := ensureTenant(ctx, tx, tenantCode)
		if err != nil {
			return err
		}

		identity, found, err := findIdentity(ctx, tx, profile)
		if err != nil {
			return err
		}

		var user userModel
		if found {
			if err := tx.Where("id = ?", identity.UserID).Take(&user).Error; err != nil {
				return err
			}
		} else {
			role := UserRoleMember
			var existingCount int64
			if err := tx.Model(&userModel{}).Where("tenant_id = ?", tenant.ID).Count(&existingCount).Error; err != nil {
				return err
			}
			if existingCount == 0 {
				role = UserRoleAdmin
			}

			user = userModel{
				TenantID:    tenant.ID,
				DisplayName: displayNameFromWeChat(profile),
				AvatarURL:   emptyStringPtr(profile.AvatarURL),
				Role:        string(role),
				Status:      string(UserStatusActive),
				LastLoginAt: &now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		}

		if user.TenantID == 0 {
			user.TenantID = tenant.ID
		}
		user.TenantID = tenant.ID
		user.DisplayName = displayNameFromWeChat(profile)
		user.AvatarURL = emptyStringPtr(profile.AvatarURL)
		user.LastLoginAt = &now
		user.UpdatedAt = now
		if user.Status == "" {
			user.Status = string(UserStatusActive)
		}
		if user.Role == "" {
			user.Role = string(UserRoleMember)
		}
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		rawProfile, err := json.Marshal(profile)
		if err != nil {
			return err
		}
		if !found {
			identity = userIdentityModel{
				UserID:         user.ID,
				Provider:       weChatProvider,
				ProviderUserID: profile.OpenID,
				CreatedAt:      now,
			}
		}
		identity.UserID = user.ID
		identity.Provider = weChatProvider
		identity.ProviderUserID = profile.OpenID
		identity.UnionID = emptyStringPtr(profile.UnionID)
		identity.RawProfile = rawProfile
		identity.UpdatedAt = now
		if identity.CreatedAt.IsZero() {
			identity.CreatedAt = now
		}
		if err := tx.Save(&identity).Error; err != nil {
			return err
		}

		result = toUser(user, tenant.TenantCode, identity)
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return result, nil
}

func (s *GormStore) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, ip string, userAgent string, now time.Time) error {
	session := userSessionModel{
		UserID:     userID,
		TokenHash:  tokenHash,
		ExpiresAt:  expiresAt,
		LastSeenAt: &now,
		LastSeenIP: emptyStringPtr(ip),
		UserAgent:  emptyStringPtr(userAgent),
		CreatedAt:  now,
	}
	return s.db.WithContext(ctx).Create(&session).Error
}

func (s *GormStore) GetUserBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (User, SessionRecord, bool, error) {
	var session userSessionModel
	err := s.db.WithContext(ctx).Where("token_hash = ?", tokenHash).Take(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, SessionRecord{}, false, nil
		}
		return User{}, SessionRecord{}, false, err
	}
	if session.ExpiresAt.Before(now) {
		return User{}, SessionRecord{}, false, nil
	}

	var joined userJoinedRow
	err = s.db.WithContext(ctx).
		Table("users").
		Select("users.id, users.display_name, users.avatar_url, users.role, users.status, users.last_login_at, users.created_at, users.updated_at, tenants.tenant_code").
		Joins("join tenants on tenants.id = users.tenant_id").
		Where("users.id = ?", session.UserID).
		Take(&joined).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, SessionRecord{}, false, nil
		}
		return User{}, SessionRecord{}, false, err
	}
	if joined.Status != string(UserStatusActive) {
		return User{}, SessionRecord{}, false, nil
	}

	if err := s.db.WithContext(ctx).Model(&userSessionModel{}).
		Where("id = ?", session.ID).
		Updates(map[string]any{
			"last_seen_at": now,
			"updated_at":   now,
		}).Error; err != nil {
		return User{}, SessionRecord{}, false, err
	}

	return User{
			ID:          joined.ID,
			TenantCode:  joined.TenantCode,
			DisplayName: joined.DisplayName,
			AvatarURL:   valueOrEmpty(joined.AvatarURL),
			Role:        UserRole(joined.Role),
			Status:      UserStatus(joined.Status),
			LastLoginAt: joined.LastLoginAt,
			CreatedAt:   joined.CreatedAt,
			UpdatedAt:   joined.UpdatedAt,
		}, SessionRecord{
			ID:        session.ID,
			UserID:    session.UserID,
			ExpiresAt: session.ExpiresAt,
		}, true, nil
}

func (s *GormStore) DeleteSession(ctx context.Context, tokenHash string) error {
	return s.db.WithContext(ctx).Where("token_hash = ?", tokenHash).Delete(&userSessionModel{}).Error
}

func (s *GormStore) ListUsers(ctx context.Context, tenantCode string) ([]User, error) {
	var rows []userJoinedRow
	err := s.db.WithContext(ctx).
		Table("users").
		Select("users.id, users.display_name, users.avatar_url, users.role, users.status, users.last_login_at, users.created_at, users.updated_at, tenants.tenant_code, user_identities.provider, user_identities.provider_user_id").
		Joins("join tenants on tenants.id = users.tenant_id").
		Joins("left join user_identities on user_identities.user_id = users.id and user_identities.provider = ?", weChatProvider).
		Where("tenants.tenant_code = ?", tenantCode).
		Order("users.created_at asc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]User, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toUser())
	}
	return items, nil
}

func (s *GormStore) UpdateUser(ctx context.Context, tenantCode string, userID int64, patch UserUpdate) (User, error) {
	var result User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tenant tenantRefModel
		if err := tx.Where("tenant_code = ?", tenantCode).Take(&tenant).Error; err != nil {
			return err
		}

		var user userModel
		if err := tx.Where("id = ? and tenant_id = ?", userID, tenant.ID).Take(&user).Error; err != nil {
			return err
		}

		if patch.DisplayName != nil {
			user.DisplayName = *patch.DisplayName
		}
		if patch.Role != nil {
			user.Role = string(*patch.Role)
		}
		if patch.Status != nil {
			user.Status = string(*patch.Status)
		}
		user.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		var identity userIdentityModel
		_ = tx.Where("user_id = ? and provider = ?", user.ID, weChatProvider).Take(&identity).Error
		result = toUser(user, tenant.TenantCode, identity)
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return result, nil
}

func findIdentity(ctx context.Context, db *gorm.DB, profile WeChatProfile) (userIdentityModel, bool, error) {
	var identity userIdentityModel
	err := db.WithContext(ctx).
		Where("provider = ? and provider_user_id = ?", weChatProvider, profile.OpenID).
		Take(&identity).Error
	if err == nil {
		return identity, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return userIdentityModel{}, false, err
	}
	if profile.UnionID == "" {
		return userIdentityModel{}, false, nil
	}
	err = db.WithContext(ctx).
		Where("provider = ? and union_id = ?", weChatProvider, profile.UnionID).
		Take(&identity).Error
	if err == nil {
		return identity, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return userIdentityModel{}, false, nil
	}
	return userIdentityModel{}, false, err
}

func ensureTenant(ctx context.Context, db *gorm.DB, tenantCode string) (tenantRefModel, error) {
	var tenant tenantRefModel
	if err := db.WithContext(ctx).Where("tenant_code = ?", tenantCode).Take(&tenant).Error; err != nil {
		return tenantRefModel{}, fmt.Errorf("load tenant %q: %w", tenantCode, err)
	}
	return tenant, nil
}

func displayNameFromWeChat(profile WeChatProfile) string {
	if profile.Nickname != "" {
		return profile.Nickname
	}
	return profile.OpenID
}

func toUser(user userModel, tenantCode string, identity userIdentityModel) User {
	return User{
		ID:             user.ID,
		TenantCode:     tenantCode,
		DisplayName:    user.DisplayName,
		AvatarURL:      valueOrEmpty(user.AvatarURL),
		Role:           UserRole(user.Role),
		Status:         UserStatus(user.Status),
		LastLoginAt:    user.LastLoginAt,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
		Provider:       identity.Provider,
		ProviderUserID: identity.ProviderUserID,
	}
}

func (r userJoinedRow) toUser() User {
	return User{
		ID:             r.ID,
		TenantCode:     r.TenantCode,
		DisplayName:    r.DisplayName,
		AvatarURL:      valueOrEmpty(r.AvatarURL),
		Role:           UserRole(r.Role),
		Status:         UserStatus(r.Status),
		LastLoginAt:    r.LastLoginAt,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		Provider:       r.Provider,
		ProviderUserID: r.ProviderUserID,
	}
}

func emptyStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type tenantRefModel struct {
	ID         int64  `gorm:"column:id;primaryKey"`
	TenantCode string `gorm:"column:tenant_code"`
}

func (tenantRefModel) TableName() string {
	return "tenants"
}

type userModel struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	TenantID    int64      `gorm:"column:tenant_id"`
	DisplayName string     `gorm:"column:display_name"`
	AvatarURL   *string    `gorm:"column:avatar_url"`
	Role        string     `gorm:"column:role"`
	Status      string     `gorm:"column:status"`
	LastLoginAt *time.Time `gorm:"column:last_login_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (userModel) TableName() string {
	return "users"
}

type userIdentityModel struct {
	ID             int64     `gorm:"column:id;primaryKey"`
	UserID         int64     `gorm:"column:user_id"`
	Provider       string    `gorm:"column:provider"`
	ProviderUserID string    `gorm:"column:provider_user_id"`
	UnionID        *string   `gorm:"column:union_id"`
	RawProfile     []byte    `gorm:"column:raw_profile"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (userIdentityModel) TableName() string {
	return "user_identities"
}

type userSessionModel struct {
	ID         int64      `gorm:"column:id;primaryKey"`
	UserID     int64      `gorm:"column:user_id"`
	TokenHash  string     `gorm:"column:token_hash"`
	ExpiresAt  time.Time  `gorm:"column:expires_at"`
	LastSeenAt *time.Time `gorm:"column:last_seen_at"`
	LastSeenIP *string    `gorm:"column:last_seen_ip"`
	UserAgent  *string    `gorm:"column:user_agent"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at"`
}

func (userSessionModel) TableName() string {
	return "user_sessions"
}

type userJoinedRow struct {
	ID             int64      `gorm:"column:id"`
	TenantCode     string     `gorm:"column:tenant_code"`
	DisplayName    string     `gorm:"column:display_name"`
	AvatarURL      *string    `gorm:"column:avatar_url"`
	Role           string     `gorm:"column:role"`
	Status         string     `gorm:"column:status"`
	LastLoginAt    *time.Time `gorm:"column:last_login_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	Provider       string     `gorm:"column:provider"`
	ProviderUserID string     `gorm:"column:provider_user_id"`
}
