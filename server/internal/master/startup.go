package master

import (
	"context"
	"os"
	"time"

	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/server/internal/master/auth"
	"gorm.io/gorm"
)

const AdminPasswordEnv = "GAOMING_ADMIN_PASSWORD"
const DefaultAdminPassword = "G@oming"

// InitializeAdmin runs before the unified server starts accepting requests.
func InitializeAdmin(ctx context.Context, db *gorm.DB, logger *logx.Logger) error {
	password := os.Getenv(AdminPasswordEnv)
	fallback := password == ""
	if fallback {
		password = DefaultAdminPassword
	}
	created, err := auth.NewStore(db).EnsureStartupAdmin(ctx, password, time.Now().UTC())
	if err != nil {
		return err
	}
	if created && logger != nil {
		logger.Info("startup administrator initialized", "username", "admin", "tenant", "default")
		if fallback {
			logger.Warn("startup administrator uses the default password; configure GAOMING_ADMIN_PASSWORD for non-local deployments")
		}
	}
	return nil
}
