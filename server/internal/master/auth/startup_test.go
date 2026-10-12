package auth

import (
	"context"
	"github.com/gofxq/gaoming/internal/testutil"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sync"
	"testing"
	"time"
)

func TestStartupAdminConcurrentAndExistingUsers(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: testutil.Postgres(t)}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	ctx := context.Background()
	if err := db.Exec("INSERT INTO auth_bootstrap_state VALUES(1,now())").Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	created := make(chan bool, 4)
	failures := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := store.EnsureStartupAdmin(ctx, "G@oming", time.Now().UTC())
			created <- ok
			failures <- err
		}()
	}
	wg.Wait()
	close(created)
	close(failures)
	count := 0
	for ok := range created {
		if ok {
			count++
		}
	}
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count != 1 {
		t.Fatalf("created %d administrators", count)
	}
	user, hash, ok, err := store.LocalCredentials(ctx, "default", "admin")
	if err != nil || !ok || user.Role != UserRoleAdmin || !VerifyPassword(hash, "G@oming") || hash == "G@oming" {
		t.Fatal("startup account invalid", err)
	}
	if ok, err := store.EnsureStartupAdmin(ctx, "replacement-password", time.Now().UTC()); ok || err != nil {
		t.Fatal("populated users table modified", err)
	}
	_, after, _, _ := store.LocalCredentials(ctx, "default", "admin")
	if after != hash {
		t.Fatal("existing password reset")
	}
	if err := db.Exec("DELETE FROM users").Error; err != nil {
		t.Fatal(err)
	}
	if ok, err := store.EnsureStartupAdmin(ctx, "replacement-password", time.Now().UTC()); !ok || err != nil {
		t.Fatal("empty users table not reinitialized", err)
	}
}
