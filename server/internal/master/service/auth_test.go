package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofxq/gaoming/internal/testutil"
	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/server/internal/master/auth"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func authService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: testutil.Postgres(t)}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return New(nil, nil, nil, nil, auth.NewStore(db), AuthConfig{SetupToken: strings.Repeat("s", 32)}, clock.Real{}, nil), db
}

func TestSetupTokenSingleUse(t *testing.T) {
	svc, db := authService(t)
	req := SetupRequest{SetupToken: strings.Repeat("s", 32), TenantCode: "team", Username: "admin", DisplayName: "Admin", Password: "correct-password"}
	bad := req
	bad.SetupToken = "wrong"
	if _, err := svc.Setup(context.Background(), bad); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("bad setup token accepted: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := svc.Setup(context.Background(), req); errs <- err }()
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent setup succeeded %d times", success)
	}
	var count int64
	db.Table("users").Count(&count)
	if count != 1 {
		t.Fatalf("created %d users", count)
	}
	if available, err := svc.SetupAvailable(context.Background()); err != nil || available {
		t.Fatalf("setup reopened: %v %v", available, err)
	}
}

func TestSetupRemainsClosedAfterUsersDeleted(t *testing.T) {
	svc, db := authService(t)
	ctx := context.Background()
	req := SetupRequest{SetupToken: strings.Repeat("s", 32), TenantCode: "team", Username: "admin", DisplayName: "Admin", Password: "correct-password"}
	if _, err := svc.Setup(ctx, req); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"user_sessions", "user_passwords", "users"} {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Reconstruct the service as a restart would, keeping the persistent marker.
	svc = New(nil, nil, nil, nil, auth.NewStore(db), AuthConfig{SetupToken: req.SetupToken}, clock.Real{}, nil)
	if available, err := svc.SetupAvailable(ctx); err != nil || available {
		t.Fatalf("setup reopened after deletion/restart: %v %v", available, err)
	}
	if _, err := svc.Setup(ctx, req); !errors.Is(err, ErrForbidden) {
		t.Fatalf("repeated setup accepted after deletion: %v", err)
	}
}

func TestLoginDisabledAndExpiredSession(t *testing.T) {
	svc, db := authService(t)
	ctx := context.Background()
	_, err := svc.Setup(ctx, SetupRequest{SetupToken: strings.Repeat("s", 32), TenantCode: "team", Username: "admin", DisplayName: "Admin", Password: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range []LoginRequest{{TenantCode: "team", Username: "admin", Password: "wrong-password"}, {TenantCode: "team", Username: "unknown", Password: "correct-password"}, {TenantCode: "other", Username: "admin", Password: "correct-password"}} {
		if _, err := svc.Login(ctx, req); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("bad login accepted: %v", err)
		}
	}
	session, err := svc.Login(ctx, LoginRequest{TenantCode: "team", Username: "admin", Password: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	user, _, ok, err := svc.ResolveSessionUser(ctx, session.Token)
	if err != nil || !ok || user.TenantCode != "team" {
		t.Fatalf("session failed: %v %v %+v", ok, err, user)
	}
	if err := db.Exec(`UPDATE users SET status='disabled'`).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := svc.ResolveSessionUser(ctx, session.Token); err != nil || ok {
		t.Fatalf("disabled session accepted: %v %v", ok, err)
	}
	if _, err := svc.Login(ctx, LoginRequest{TenantCode: "team", Username: "admin", Password: "correct-password"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("disabled login accepted: %v", err)
	}
	db.Exec(`UPDATE users SET status='active'`)
	db.Exec(`UPDATE user_sessions SET expires_at=?`, time.Now().Add(-time.Second))
	if _, _, ok, err := svc.ResolveSessionUser(ctx, session.Token); err != nil || ok {
		t.Fatalf("expired session accepted: %v %v", ok, err)
	}
	session, err = svc.Login(ctx, LoginRequest{TenantCode: "team", Username: "admin", Password: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(ctx, session.Token); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := svc.ResolveSessionUser(ctx, session.Token); err != nil || ok {
		t.Fatalf("logged out session accepted: %v %v", ok, err)
	}
}
