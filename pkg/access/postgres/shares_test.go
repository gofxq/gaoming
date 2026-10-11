package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gofxq/gaoming/internal/testutil"
	"github.com/gofxq/gaoming/pkg/access"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestShareScopeExpiryAndRevocation(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: testutil.Postgres(t)}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"team-a", "team-b"} {
		if err := db.Exec(`INSERT INTO tenants(tenant_code,name) VALUES (?,?)`, code, code).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO users(tenant_id,display_name,role,status) SELECT id,'Admin','admin','active' FROM tenants WHERE tenant_code=?`, code).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO hosts(tenant_id,host_uid,hostname,primary_ip,os_type,arch,region) SELECT id,?,'private-name','127.0.0.1','linux','amd64','test' FROM tenants WHERE tenant_code=?`, code+"-host", code).Error; err != nil {
			t.Fatal(err)
		}
	}
	var id int64
	db.Raw(`SELECT users.id FROM users JOIN tenants ON tenants.id=tenant_id WHERE tenant_code='team-a'`).Scan(&id)
	p := access.Principal{Kind: access.User, TenantCode: "team-a", UserID: id, Role: "admin"}
	ctx := context.Background()
	now := time.Now().UTC()
	store := NewStore(db)
	token, hash, err := access.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	req := access.CreateShareRequest{Title: "Incident", HostUIDs: []string{"team-a-host"}, Fields: []string{"status"}, TTLSeconds: 3600}
	share, err := store.CreateShare(ctx, p, req, hash, now)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	db.Raw(`SELECT token_hash FROM access_shares WHERE id=?`, share.ID).Scan(&stored)
	if stored == token || stored != hash {
		t.Fatal("raw share token stored or incorrect hash")
	}
	grant, err := store.Authenticate(ctx, token, access.Share, now)
	if err != nil {
		t.Fatal(err)
	}
	if !grant.AllowsHost("team-a", "team-a-host") || grant.AllowsHost("team-b", "team-b-host") || grant.AllowsHost("team-a", "not-shared") || grant.AllowsField("hostname") {
		t.Fatalf("incorrect share scope: %+v", grant)
	}
	if _, err := store.Authenticate(ctx, token, access.Agent, now); !errors.Is(err, access.ErrUnauthorized) {
		t.Fatalf("share token accepted as agent: %v", err)
	}
	if _, err := store.Authenticate(ctx, token, access.Share, now.Add(time.Hour)); !errors.Is(err, access.ErrUnauthorized) {
		t.Fatalf("expired token accepted: %v", err)
	}
	session, err := store.ExchangeShare(ctx, token, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateShareSession(ctx, session.Token, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateShareSession(ctx, session.Token, now.Add(time.Hour)); !errors.Is(err, access.ErrUnauthorized) {
		t.Fatalf("expired share cookie accepted: %v", err)
	}
	for _, change := range []string{"status='disabled'", "role='member'"} {
		if err := db.Exec(`UPDATE users SET `+change+` WHERE id=?`, id).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := store.AuthenticateShareSession(ctx, session.Token, now); !errors.Is(err, access.ErrUnauthorized) {
			t.Fatalf("creator lost authority but cookie remains valid: %v", err)
		}
		if err := db.Exec(`UPDATE users SET status='active',role='admin' WHERE id=?`, id).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RevokeShare(ctx, p, share.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(ctx, token, access.Share, now); !errors.Is(err, access.ErrUnauthorized) {
		t.Fatalf("revoked link accepted: %v", err)
	}
	if _, err := store.AuthenticateShareSession(ctx, session.Token, now); !errors.Is(err, access.ErrUnauthorized) {
		t.Fatalf("revoked session accepted: %v", err)
	}
	req.HostUIDs = []string{"team-b-host"}
	if _, err := store.CreateShare(ctx, p, req, hash, now); !errors.Is(err, access.ErrNotFound) {
		t.Fatalf("cross-tenant host shared: %v", err)
	}
	req.HostUIDs = []string{"team-a-host"}
	req.Fields = []string{"primary_ip"}
	if _, err := store.CreateShare(ctx, p, req, hash, now); err == nil {
		t.Fatal("private field allowed")
	}
	if err := store.RevokeShare(ctx, p, "missing", now); !errors.Is(err, access.ErrNotFound) {
		t.Fatalf("missing share revoked: %v", err)
	}
	// Race redemption against revocation. Regardless of which commits first,
	// all issued cookies must be unusable after the revoke returns.
	req.Fields = []string{"status"}
	for i := 0; i < 3; i++ {
		raw, hash, err := access.NewToken()
		if err != nil {
			t.Fatal(err)
		}
		record, err := store.CreateShare(ctx, p, req, hash, now)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		tokens := make(chan string, 5)
		errs := make(chan error, 6)
		for j := 0; j < 5; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				session, err := store.ExchangeShare(ctx, raw, now)
				if err == nil {
					tokens <- session.Token
				} else if !errors.Is(err, access.ErrUnauthorized) {
					errs <- err
				}
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := store.RevokeShare(ctx, p, record.ID, now); err != nil {
				errs <- err
			}
		}()
		close(start)
		wg.Wait()
		close(tokens)
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}
		for raw := range tokens {
			if _, err := store.AuthenticateShareSession(ctx, raw, now); !errors.Is(err, access.ErrUnauthorized) {
				t.Fatalf("race left an authorized cookie: %v", err)
			}
		}
	}
	// Exercise the storage quotas directly; invalid identities cannot multiply
	// rows by bypassing a browser or a single process's rate limiter.
	raw, hash, err := access.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.CreateShare(ctx, p, req, hash, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO access_share_sessions(token_hash,share_id,expires_at,created_at)
		SELECT lpad(n::text,64,'0'),?,?,? FROM generate_series(1,100) n`, record.ID, now.Add(time.Hour), now).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExchangeShare(ctx, raw, now); !errors.Is(err, access.ErrQuota) {
		t.Fatalf("session quota exceeded: %v", err)
	}
	var current int64
	db.Table("access_shares").Count(&current)
	if err := db.Exec(`INSERT INTO access_shares(id,tenant_id,title,token_hash,fields,expires_at,created_by,created_at)
		SELECT 'quota-'||n,tenant_id,'Quota',lpad(n::text,64,'0'),'["status"]'::jsonb,?,?,? FROM users CROSS JOIN generate_series(1,?) n WHERE users.id=?`, now.Add(time.Hour), id, now, 100-current, id).Error; err != nil {
		t.Fatal(err)
	}
	_, hash, err = access.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateShare(ctx, p, req, hash, now); !errors.Is(err, access.ErrQuota) {
		t.Fatalf("share quota exceeded: %v", err)
	}
}
