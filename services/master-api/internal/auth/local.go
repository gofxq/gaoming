package auth

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrSetupUnavailable = errors.New("administrator setup unavailable")

func (s *GormStore) SetupAvailable(ctx context.Context) (bool, error) {
	var available bool
	err := s.db.WithContext(ctx).Raw(`SELECT NOT EXISTS(SELECT 1 FROM auth_bootstrap_state)
		AND NOT EXISTS(SELECT 1 FROM users)`).Scan(&available).Error
	return available, err
}

func (s *GormStore) BootstrapAdmin(ctx context.Context, account LocalAccount, now time.Time) (User, error) {
	var user User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(291013431)`).Error; err != nil {
			return err
		}
		var available bool
		if err := tx.Raw(`SELECT NOT EXISTS(SELECT 1 FROM auth_bootstrap_state)
			AND NOT EXISTS(SELECT 1 FROM users)`).Scan(&available).Error; err != nil {
			return err
		}
		if !available {
			return ErrSetupUnavailable
		}
		var err error
		user, err = createLocalAdmin(tx, account, now)
		if err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO auth_bootstrap_state(id,completed_at) VALUES (1,?)`, now).Error
	})
	return user, err
}

func (s *GormStore) CreateLocalAdmin(ctx context.Context, account LocalAccount, now time.Time) (User, error) {
	var user User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize with first-time setup, including local administrative recovery.
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(291013431)`).Error; err != nil {
			return err
		}
		var err error
		user, err = createLocalAdmin(tx, account, now)
		if err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO auth_bootstrap_state(id,completed_at) VALUES (1,?) ON CONFLICT DO NOTHING`, now).Error
	})
	return user, err
}

func createLocalAdmin(tx *gorm.DB, account LocalAccount, now time.Time) (User, error) {
	if err := tx.Exec(`INSERT INTO tenants(tenant_code,name) VALUES (?,?) ON CONFLICT(tenant_code) DO NOTHING`, account.TenantCode, account.TenantCode).Error; err != nil {
		return User{}, err
	}
	var tenant tenantRefModel
	if err := tx.Where("tenant_code = ? AND status = 1", account.TenantCode).Take(&tenant).Error; err != nil {
		return User{}, err
	}
	user := userModel{TenantID: tenant.ID, DisplayName: account.DisplayName, Role: string(UserRoleAdmin), Status: string(UserStatusActive), CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&user).Error; err != nil {
		return User{}, err
	}
	if err := tx.Exec(`INSERT INTO user_passwords(user_id,tenant_id,username,password_hash) VALUES (?,?,?,?)`, user.ID, tenant.ID, account.Username, account.PasswordHash).Error; err != nil {
		return User{}, err
	}
	return toUser(user, account.TenantCode, userIdentityModel{}), nil
}

func (s *GormStore) LocalCredentials(ctx context.Context, tenant, username string) (User, string, bool, error) {
	var row struct {
		Joined       userJoinedRow `gorm:"embedded"`
		PasswordHash string
	}
	err := s.db.WithContext(ctx).Table("user_passwords").Select(`users.id, users.display_name,
		users.avatar_url, users.role, users.status, users.last_login_at, users.created_at,
		users.updated_at, tenants.tenant_code, user_passwords.password_hash`).
		Joins("JOIN users ON users.id=user_passwords.user_id AND users.tenant_id=user_passwords.tenant_id").
		Joins("JOIN tenants ON tenants.id=users.tenant_id").
		Where("tenants.tenant_code=? AND user_passwords.username=? AND tenants.status=1", tenant, username).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, "", false, nil
	}
	if err != nil {
		return User{}, "", false, err
	}
	return row.Joined.toUser(), row.PasswordHash, true, nil
}
