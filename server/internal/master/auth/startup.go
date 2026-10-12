package auth

import (
	"context"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"time"
)

// EnsureStartupAdmin follows the current global users table, not the historical
// web setup marker. The short password exception is limited to this startup path.
func (s *GormStore) EnsureStartupAdmin(ctx context.Context, password string, now time.Time) (bool, error) {
	created := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(291013431)`).Error; err != nil {
			return err
		}
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
