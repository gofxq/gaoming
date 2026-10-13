package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "master-api.yml")
	content := strings.Join([]string{
		"http_addr: ':18080'",
		"runtime_backend: pg_redis",
		"postgres_dsn: postgres://demo:demo@127.0.0.1:5432/demo?sslmode=disable",
		"redis_addr: 127.0.0.1:6379",
		"redis_password: secret",
		"redis_db: 3",
		"tenant_code: test",
		"tenant_name: Test Tenant",
		"allow_custom_tenant_code: false",
		"session_cookie_name: demo_session",
		"session_secret: super-secret",
		"session_ttl_hours: 24",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.HTTPAddr != ":18080" {
		t.Fatalf("unexpected http addr: %q", cfg.HTTPAddr)
	}
	if cfg.RedisDB != 3 {
		t.Fatalf("unexpected redis db: %d", cfg.RedisDB)
	}
	if cfg.SessionTTLHours != 24 {
		t.Fatalf("unexpected session ttl: %d", cfg.SessionTTLHours)
	}
	if cfg.AllowCustomTenantCode {
		t.Fatal("expected allow_custom_tenant_code to be false")
	}
}

func TestLoadFromFileAppliesDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "master-api.yml")
	if err := os.WriteFile(path, []byte("postgres_dsn: postgres://demo:demo@127.0.0.1:5432/demo?sslmode=disable\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("unexpected default http addr: %q", cfg.HTTPAddr)
	}
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Fatalf("unexpected default redis addr: %q", cfg.RedisAddr)
	}
	if cfg.SessionCookieName != "gaoming_session" {
		t.Fatalf("unexpected default session cookie: %q", cfg.SessionCookieName)
	}
}
