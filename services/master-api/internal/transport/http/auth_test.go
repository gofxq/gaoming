package http

import (
	"bytes"
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofxq/gaoming/internal/testutil"
	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/master-api/internal/auth"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestLoginOriginAndPasswordBoundaries(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: testutil.Postgres(t)}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(nil, nil, nil, nil, auth.NewStore(db), service.AuthConfig{SetupToken: strings.Repeat("s", 32), CookieSecure: true, AllowedOrigins: []string{"https://monitor.example"}}, clock.Real{}, logx.New("auth-test"))
	handler := NewServer(svc, logx.New("auth-test")).Handler()
	request := func(path, body, origin, csrf string, cookie *nethttp.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(nethttp.MethodPost, path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	body := `{"setup_token":"` + strings.Repeat("s", 32) + `","tenant_code":"team","username":"admin","display_name":"Admin","password":"correct-password"}`
	for _, origin := range []string{"", "https://evil.example", "null"} {
		if w := request("/master/api/v1/auth/setup", body, origin, "", nil); w.Code != 403 {
			t.Fatalf("invalid origin accepted: %s %d", origin, w.Code)
		}
	}
	w := request("/master/api/v1/auth/setup", body, "https://monitor.example", "", nil)
	if w.Code != 201 {
		t.Fatalf("setup failed: %d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing session cookie")
	}
	cookie := cookies[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != nethttp.SameSiteLaxMode || cookie.MaxAge <= 0 {
		t.Fatalf("unsafe cookie: %+v", cookie)
	}
	if strings.Contains(w.Body.String(), cookie.Value) || strings.Contains(w.Body.String(), "password") {
		t.Fatal("session JSON leaks credential")
	}
	if w := request("/master/api/v1/auth/logout", `{}`, "https://monitor.example", "", cookie); w.Code != 403 {
		t.Fatalf("logout without CSRF accepted: %d", w.Code)
	}
	r := httptest.NewRequest(nethttp.MethodGet, "/master/api/v1/auth/csrf", nil)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Token == "" || payload.Token == cookie.Value {
		t.Fatal("invalid CSRF token")
	}
	if w := request("/master/api/v1/auth/logout", `{}`, "https://evil.example", payload.Token, cookie); w.Code != 403 {
		t.Fatalf("cross-origin logout accepted: %d", w.Code)
	}
	if w := request("/master/api/v1/auth/logout", `{}`, "https://monitor.example", payload.Token, cookie); w.Code != 204 {
		t.Fatalf("logout failed: %d %s", w.Code, w.Body.String())
	}
	if _, _, ok, err := svc.ResolveSessionUser(context.Background(), cookie.Value); err != nil || ok {
		t.Fatalf("logout session survived: %v %v", ok, err)
	}
}
