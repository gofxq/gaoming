package auth

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// EnsureStartupAdmin initializes admin/default only when the entire users table
// is empty. It deliberately ignores a historical web-bootstrap marker: startup
// initialization follows the current user table, including after all users are deleted.
func (s *GormStore) EnsureStartupAdmin(ctx context.Context, password string, now time.Time) (bool, error) {
	created := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Use the same lock/order as web setup and the administrator CLI.
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(291013431)`).Error; err != nil {
			return err
		}
		// Also serialize user inserts/deletes that do not use the advisory lock.
		if err := tx.Exec(`LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE`).Error; err != nil {
			return err
		}
		var exists bool
		if err := tx.Raw(`SELECT EXISTS(SELECT 1 FROM users)`).Scan(&exists).Error; err != nil {
			return err
		}
		if exists {
			return nil
		}
		if len(password) == 0 || len(password) > 72 {
			return fmt.Errorf("startup administrator password must contain 1 to 72 bytes")
		}
		// This startup-only path accepts the explicitly configured short fallback;
		// ordinary user creation retains HashPassword's 12-byte minimum.
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return err
		}
		if _, err := createLocalAdmin(tx, LocalAccount{TenantCode: "default", Username: "admin", DisplayName: "Administrator", PasswordHash: string(hash)}, now); err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO auth_bootstrap_state(id,completed_at) VALUES (1,?) ON CONFLICT DO NOTHING`, now).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return created, nil
}
