package auth

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofxq/gaoming/internal/testutil"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func startupStore(t *testing.T) *GormStore {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: testutil.Postgres(t)}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return NewStore(db)
}

func TestStartupAdminDefaultPasswordAndConcurrentInitialization(t *testing.T) {
	store := startupStore(t)
	ctx := context.Background()
	// Startup must follow the empty user table even if web setup previously completed.
	if err := store.db.Exec("INSERT INTO auth_bootstrap_state(id,completed_at) VALUES (1,now())").Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	created := make(chan bool, 6)
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := store.EnsureStartupAdmin(ctx, "G@oming", time.Now().UTC())
			created <- ok
			errs <- err
		}()
	}
	wg.Wait()
	close(created)
	close(errs)
	count := 0
	for ok := range created {
		if ok {
			count++
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count != 1 {
		t.Fatalf("created %d startup administrators", count)
	}
	user, hash, ok, err := store.LocalCredentials(ctx, "default", "admin")
	if err != nil || !ok {
		t.Fatal("startup account missing", err)
	}
	if user.Role != UserRoleAdmin || user.Status != UserStatusActive || user.TenantCode != "default" || hash == "G@oming" || !VerifyPassword(hash, "G@oming") {
		t.Fatal("startup admin role, tenant or password invalid")
	}
	if ok, err := store.EnsureStartupAdmin(ctx, "replacement-password", time.Now().UTC()); err != nil || ok {
		t.Fatal("existing account was replaced", err)
	}
	_, unchanged, _, _ := store.LocalCredentials(ctx, "default", "admin")
	if hash != unchanged {
		t.Fatal("restart reset administrator password")
	}
	if err := store.db.Exec("DELETE FROM users").Error; err != nil {
		t.Fatal(err)
	}
	if created, err := store.EnsureStartupAdmin(ctx, "recreated-password", time.Now().UTC()); err != nil || !created {
		t.Fatal("empty user table was not reinitialized", err)
	}
	recreated, newHash, ok, err := store.LocalCredentials(ctx, "default", "admin")
	if err != nil || !ok || recreated.ID == user.ID || !VerifyPassword(newHash, "recreated-password") {
		t.Fatal("recreated administrator is invalid", err)
	}

}

func TestStartupAdminSkipsAnyExistingTenantUser(t *testing.T) {
	store := startupStore(t)
	ctx := context.Background()
	hash, err := HashPassword("existing-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BootstrapAdmin(ctx, LocalAccount{TenantCode: "other", Username: "existing", DisplayName: "Existing", PasswordHash: hash}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if created, err := store.EnsureStartupAdmin(ctx, strings.Repeat("x", 73), time.Now().UTC()); err != nil || created {
		t.Fatal("startup touched a populated user table", err)
	}
	if _, _, ok, err := store.LocalCredentials(ctx, "default", "admin"); err != nil || ok {
		t.Fatal("unrequested default account created", err)
	}
}

func TestStartupAdminRejectsInvalidPasswordAtomically(t *testing.T) {
	for _, password := range []string{"", strings.Repeat("x", 73)} {
		store := startupStore(t)
		if created, err := store.EnsureStartupAdmin(context.Background(), password, time.Now().UTC()); err == nil || created {
			t.Fatal("invalid startup password accepted")
		}
		var count int64
		if err := store.db.Table("users").Count(&count).Error; err != nil || count != 0 {
			t.Fatal("failed initialization left a user", err)
		}
	}
}
