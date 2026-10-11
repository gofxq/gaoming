package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gofxq/gaoming/internal/testutil"
	"github.com/gofxq/gaoming/pkg/access"
	"github.com/gofxq/gaoming/pkg/contracts"
	hostruntime "github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOpsUseSessionTenantAndActor(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: testutil.Postgres(t)}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store, err := NewStore(ctx, db, Config{TenantCode: "default", TenantName: "Default"})
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
		if err := db.Exec(`INSERT INTO hosts(tenant_id,host_uid,hostname,primary_ip,os_type,arch,region) SELECT id,?,'host','127.0.0.1','linux','amd64','test' FROM tenants WHERE tenant_code=?`, code+"-host", code).Error; err != nil {
			t.Fatal(err)
		}
	}
	var userID int64
	if err := db.Raw(`SELECT users.id FROM users JOIN tenants ON tenants.id=users.tenant_id WHERE tenant_code='team-a'`).Scan(&userID).Error; err != nil {
		t.Fatal(err)
	}
	principal := access.Principal{Kind: access.User, TenantCode: "team-a", UserID: userID, Role: "admin"}
	req := contracts.CreateMaintenanceWindowRequest{Title: "Upgrade", ScopeType: "host", ScopeRef: "team-a-host", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), CreatedBy: "forged-operator"}
	window, err := store.CreateMaintenance(ctx, principal, req)
	if err != nil {
		t.Fatal(err)
	}
	if window.CreatedBy != fmt.Sprintf("user:%d", userID) {
		t.Fatalf("forged actor persisted: %s", window.CreatedBy)
	}
	var tenant string
	if err := db.Raw(`SELECT tenant_code FROM maintenance_windows JOIN tenants ON tenants.id=tenant_id WHERE maintenance_windows.id=?`, window.ID).Scan(&tenant).Error; err != nil || tenant != "team-a" {
		t.Fatalf("wrong tenant: %s %v", tenant, err)
	}
	req.ScopeRef = "team-b-host"
	if _, err := store.CreateMaintenance(ctx, principal, req); !errors.Is(err, hostruntime.ErrResourceNotFound) {
		t.Fatalf("cross-tenant scope accepted: %v", err)
	}
	if err := store.AckAlert(ctx, principal, "99999", time.Now()); !errors.Is(err, hostruntime.ErrResourceNotFound) {
		t.Fatalf("missing alert acknowledged: %v", err)
	}
	for _, code := range []string{"team-a", "team-b"} {
		if err := db.Exec(`INSERT INTO alert_rules(tenant_id,name,scope_type,expr,severity) SELECT id,'offline','tenant','offline',2 FROM tenants WHERE tenant_code=?`, code).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO alert_events(tenant_id,rule_id,fingerprint,status,severity,summary,first_fired_at,last_fired_at) SELECT alert_rules.tenant_id,alert_rules.id,?,1,2,'Offline',now(),now() FROM alert_rules JOIN tenants ON tenants.id=tenant_id WHERE tenant_code=?`, code+"-alert", code).Error; err != nil {
			t.Fatal(err)
		}
	}
	var alertID int64
	db.Raw(`SELECT id FROM alert_events WHERE fingerprint='team-a-alert'`).Scan(&alertID)
	if err := store.AckAlert(ctx, principal, fmt.Sprint(alertID), time.Now()); err != nil {
		t.Fatal(err)
	}
	var actor string
	db.Raw(`SELECT acked_by FROM alert_events WHERE id=?`, alertID).Scan(&actor)
	if actor != fmt.Sprintf("user:%d", userID) {
		t.Fatalf("bad alert actor: %s", actor)
	}
	db.Raw(`SELECT id FROM alert_events WHERE fingerprint='team-b-alert'`).Scan(&alertID)
	if err := store.AckAlert(ctx, principal, fmt.Sprint(alertID), time.Now()); !errors.Is(err, hostruntime.ErrResourceNotFound) {
		t.Fatalf("cross-tenant alert accepted: %v", err)
	}
	var audits int64
	db.Table("audit_logs").Where("actor = ?", fmt.Sprintf("user:%d", userID)).Count(&audits)
	if audits != 2 {
		t.Fatalf("expected two committed operation audit records: %d", audits)
	}
	if err := db.Exec(`UPDATE users SET role='member' WHERE id=?`, userID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateMaintenance(ctx, principal, req); !errors.Is(err, access.ErrForbidden) {
		t.Fatalf("stale administrator authority accepted: %v", err)
	}
	principal.Role = "member"
	if _, err := store.CreateMaintenance(ctx, principal, req); !errors.Is(err, access.ErrForbidden) {
		t.Fatalf("member can create maintenance: %v", err)
	}
}
