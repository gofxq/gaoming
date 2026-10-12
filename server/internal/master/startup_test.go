package master

import (
	"context"
	"testing"

	"github.com/gofxq/gaoming/internal/testutil"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/server/internal/master/auth"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestInitializeAdminReadsEnvironmentAndFallback(t *testing.T) {
	for _, value := range []string{"", "environment-password"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(AdminPasswordEnv, value)
			db, err := gorm.Open(postgres.New(postgres.Config{Conn: testutil.Postgres(t)}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			if err := InitializeAdmin(context.Background(), db, logx.NewNop()); err != nil {
				t.Fatal(err)
			}
			user, hash, ok, err := auth.NewStore(db).LocalCredentials(context.Background(), "default", "admin")
			if err != nil || !ok || user.Role != auth.UserRoleAdmin {
				t.Fatal("administrator initialization failed", err)
			}
			expected := value
			if expected == "" {
				expected = "G@oming"
			}
			if !auth.VerifyPassword(hash, expected) {
				t.Fatal("environment or fallback password ignored")
			}
		})
	}
}
