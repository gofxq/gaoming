package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.yml")
	if err := os.WriteFile(path, []byte("session_cookie_secure: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Master.HTTPAddr != "0.0.0.0:8080" || cfg.GRPCAddr != "0.0.0.0:8091" || cfg.CoreIntervalSec != 15 || cfg.ProbeIntervalSec != 15 {
		t.Fatalf("unexpected defaults: http=%s grpc=%s core=%d probe=%d", cfg.Master.HTTPAddr, cfg.GRPCAddr, cfg.CoreIntervalSec, cfg.ProbeIntervalSec)
	}
	if cfg.Master.SessionCookieSecure {
		t.Fatal("HTTP development setting ignored")
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	for _, body := range []string{"http_addr: bad\n", "grpc_addr: bad\n", "http_addr: '0.0.0.0:99999'\n", "core_interval_sec: -1\n", "probe_interval_sec: -1\n", "probe_target_url: file:///etc/passwd\n"} {
		t.Run(body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "server.yml")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}

func TestLoadNamespaceAndListenerOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.yml")
	if err := os.WriteFile(path, []byte("redis_namespace: test-instance\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GAOMING_HTTP_ADDR", "0.0.0.0:18080")
	t.Setenv("GAOMING_GRPC_ADDR", "0.0.0.0:18091")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RedisNamespace != "test-instance" || cfg.Master.HTTPAddr != "0.0.0.0:18080" || cfg.GRPCAddr != "0.0.0.0:18091" {
		t.Fatal("namespace or listener overrides ignored")
	}
}
