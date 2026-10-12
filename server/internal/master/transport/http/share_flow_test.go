package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofxq/gaoming/internal/testutil"
	"github.com/gofxq/gaoming/pkg/access"
	accesspg "github.com/gofxq/gaoming/pkg/access/postgres"
	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/contracts"
	hostpg "github.com/gofxq/gaoming/pkg/hostruntime/repository/postgres"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/server/internal/master/auth"
	"github.com/gofxq/gaoming/server/internal/master/service"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSharedHTTPProjectionAndRevokedStream(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: testutil.Postgres(t)}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	hosts, err := hostpg.NewStore(ctx, db, hostpg.Config{TenantCode: "team-a", TenantName: "Team A", AllowCustomTenantCode: true})
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(hosts, stubMetricStore{}, hosts, stubEventBus{}, auth.NewStore(db), service.AuthConfig{}, clock.Real{}, logx.NewNop())
	svc.ConfigureAccess(accesspg.NewStore(db), access.Principal{})
	hash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	user, err := auth.NewStore(db).BootstrapAdmin(ctx, auth.LocalAccount{TenantCode: "team-a", Username: "admin", DisplayName: "Admin", PasswordHash: hash}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, uid := range []string{"shared-host", "secret-host"} {
		_, _, _, err := hosts.RegisterAgent(ctx, contracts.RegisterAgentRequest{Host: contracts.HostIdentity{HostUID: uid, TenantCode: "team-a", Hostname: "private-inventory", PrimaryIP: "192.0.2.9", OSType: "linux", Arch: "amd64", Region: "private-region", Labels: map[string]string{"secret": "private-label"}}, Agent: contracts.AgentMetadata{AgentID: "agent-" + uid, Version: "test"}}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := hosts.Heartbeat(ctx, contracts.HeartbeatRequest{HostUID: "shared-host", AgentID: "agent-shared-host", Seq: 1, TS: time.Now(), Digest: contracts.AgentDigest{CPUUsagePct: 12, MemUsedPct: 34}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	privateSession, err := svc.Login(ctx, service.LoginRequest{TenantCode: "team-a", Username: "admin", Password: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	serverForPrivate := NewServer(svc, logx.NewNop())
	for _, test := range []struct {
		path   string
		status int
	}{{"/hosts", 200}, {"/hosts?tenant=team-b", 403}, {"/stream/hosts?tenant=team-b", 403}} {
		r := httptest.NewRequest("GET", "/master/api/v1"+test.path, nil)
		r.AddCookie(&nethttp.Cookie{Name: "gaoming_session", Value: privateSession.Token})
		w := httptest.NewRecorder()
		serverForPrivate.Handler().ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("real session scope failed: %s %d", test.path, w.Code)
		}
	}
	created, err := svc.CreateShare(ctx, access.Principal{Kind: access.User, TenantCode: user.TenantCode, UserID: user.ID, Role: "admin"}, access.CreateShareRequest{Title: "Status only", HostUIDs: []string{"shared-host"}, Fields: []string{"status"}, TTLSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(svc, logx.NewNop())
	web := httptest.NewServer(server.Handler())
	defer web.Close()
	request, err := nethttp.NewRequest("POST", web.URL+"/master/api/v1/public/shares/exchange", bytes.NewBufferString(`{"token":"`+created.Token+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", web.URL)
	response, err := web.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("exchange failed: %d", response.StatusCode)
	}
	var cookie *nethttp.Cookie
	for _, item := range response.Cookies() {
		if item.Name == "gaoming_share" {
			cookie = item
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.Path != "/master/api/v1/public/shares" || cookie.SameSite != nethttp.SameSiteLaxMode {
		t.Fatalf("incorrect scoped cookie: %+v", cookie)
	}
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/master/api/v1"+path, nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, r)
		return w
	}
	if w := get("/hosts"); w.Code != 401 {
		t.Fatalf("share cookie grants private access: %d", w.Code)
	}
	w := get("/public/shares/hosts")
	if w.Code != 200 {
		t.Fatalf("shared list failed: %d %s", w.Code, w.Body.String())
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 1 || payload.Items[0]["host_uid"] != "shared-host" {
		t.Fatalf("host scope leaked: %s", w.Body.String())
	}
	for _, name := range []string{"tenant_code", "hostname", "primary_ip", "cpu_usage_pct", "labels"} {
		if _, exists := payload.Items[0][name]; exists {
			t.Fatalf("private field leaked: %s", name)
		}
	}
	if strings.Contains(w.Body.String(), "private-") {
		t.Fatalf("inventory leaked: %s", w.Body.String())
	}
	if w := get("/public/shares/hosts/secret-host"); w.Code != 404 {
		t.Fatalf("unshared host visible: %d", w.Code)
	}
	if w := get("/public/shares/hosts/shared-host/metrics/latest"); w.Code != 403 {
		t.Fatalf("unshared metrics visible: %d", w.Code)
	}
	streamCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	r, _ := nethttp.NewRequestWithContext(streamCtx, "GET", web.URL+"/master/api/v1/public/shares/stream/hosts", nil)
	r.AddCookie(cookie)
	stream, err := web.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	if stream.StatusCode != 200 {
		t.Fatalf("stream rejected: %d", stream.StatusCode)
	}
	if err := svc.RevokeShare(ctx, access.Principal{Kind: access.User, TenantCode: user.TenantCode, UserID: user.ID, Role: "admin"}, created.ID); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(stream.Body)
	if err != nil {
		t.Fatalf("revoked stream did not terminate promptly: %v", err)
	}
	if strings.Contains(string(body), "private-") {
		t.Fatalf("private stream fields leaked: %s", body)
	}
	if w := get("/public/shares/hosts"); w.Code != 401 {
		t.Fatalf("revoked cookie still readable: %d", w.Code)
	}
	metricsShare, err := svc.CreateShare(ctx, access.Principal{Kind: access.User, TenantCode: user.TenantCode, UserID: user.ID, Role: "admin"}, access.CreateShareRequest{Title: "Summary", HostUIDs: []string{"shared-host"}, Fields: []string{"metrics"}, TTLSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	metricsSession, err := svc.ExchangeShare(ctx, metricsShare.Token)
	if err != nil {
		t.Fatal(err)
	}
	rm := httptest.NewRequest("GET", "/master/api/v1/public/shares/hosts/shared-host/metrics/latest", nil)
	rm.AddCookie(&nethttp.Cookie{Name: shareCookieName, Value: metricsSession.Token})
	wm := httptest.NewRecorder()
	server.Handler().ServeHTTP(wm, rm)
	if wm.Code != 200 || !strings.Contains(wm.Body.String(), "cpu_usage_pct") || strings.Contains(wm.Body.String(), "private-") || strings.Contains(wm.Body.String(), "labels") {
		t.Fatalf("summary projection incorrect: %d %s", wm.Code, wm.Body.String())
	}
	for _, condition := range []string{"cookie-expiry", "share-expiry", "creator-disabled", "creator-demoted"} {
		created, err := svc.CreateShare(ctx, access.Principal{Kind: access.User, TenantCode: user.TenantCode, UserID: user.ID, Role: "admin"}, access.CreateShareRequest{Title: "Revalidation", HostUIDs: []string{"shared-host"}, Fields: []string{"status"}, TTLSeconds: 60})
		if err != nil {
			t.Fatal(err)
		}
		session, err := svc.ExchangeShare(ctx, created.Token)
		if err != nil {
			t.Fatal(err)
		}
		streamCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		r, _ := nethttp.NewRequestWithContext(streamCtx, "GET", web.URL+"/master/api/v1/public/shares/stream/hosts", nil)
		r.AddCookie(&nethttp.Cookie{Name: shareCookieName, Value: session.Token})
		response, err := web.Client().Do(r)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		switch condition {
		case "cookie-expiry":
			err = db.Exec(`UPDATE access_share_sessions SET expires_at=now() WHERE share_id=?`, created.ID).Error
		case "share-expiry":
			err = db.Exec(`UPDATE access_shares SET expires_at=now() WHERE id=?`, created.ID).Error
		case "creator-disabled":
			err = db.Exec(`UPDATE users SET status='disabled' WHERE id=?`, user.ID).Error
		case "creator-demoted":
			err = db.Exec(`UPDATE users SET role='member' WHERE id=?`, user.ID).Error
		}
		if err != nil {
			cancel()
			response.Body.Close()
			t.Fatal(err)
		}
		_, err = io.ReadAll(response.Body)
		response.Body.Close()
		cancel()
		if err != nil {
			t.Fatalf("%s left old stream open: %v", condition, err)
		}
		if err := db.Exec(`UPDATE users SET status='active',role='admin' WHERE id=?`, user.ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	privateCtx, privateCancel := context.WithTimeout(ctx, 4*time.Second)
	defer privateCancel()
	privateRequest, _ := nethttp.NewRequestWithContext(privateCtx, "GET", web.URL+"/master/api/v1/stream/hosts", nil)
	privateRequest.AddCookie(&nethttp.Cookie{Name: "gaoming_session", Value: privateSession.Token})
	privateStream, err := web.Client().Do(privateRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer privateStream.Body.Close()
	if err := svc.Logout(ctx, privateSession.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(privateStream.Body); err != nil {
		t.Fatalf("logged-out private stream survived: %v", err)
	}
	svc.ConfigureAccess(accesspg.NewStore(db), access.Principal{Kind: access.Demo, TenantCode: "team-a", AllowedHosts: []string{"shared-host"}, Fields: []string{"status"}})
	if w := get("/public/demo/hosts"); w.Code != 200 || strings.Contains(w.Body.String(), "secret-host") || strings.Contains(w.Body.String(), "private-") {
		t.Fatalf("demo scope leaked: %d %s", w.Code, w.Body.String())
	}
	if err := db.Exec(`UPDATE tenants SET status=0 WHERE tenant_code='team-a'`).Error; err != nil {
		t.Fatal(err)
	}
	if w := get("/public/demo/hosts"); w.Code != 403 {
		t.Fatalf("disabled demo tenant readable: %d", w.Code)
	}
}
