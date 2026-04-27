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
	path := filepath.Join(dir, "probe-worker.yml")
	content := strings.Join([]string{
		"worker_id: probe-east-1",
		"target_url: http://127.0.0.1:8080/master",
		"report_url: http://127.0.0.1:8090/ingest",
		"region: east",
		"probe_interval_sec: 20",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.TargetURL != "http://127.0.0.1:8080/master/healthz" {
		t.Fatalf("unexpected target url: %q", cfg.TargetURL)
	}
	if cfg.ReportURL != "http://127.0.0.1:8090/ingest/api/v1/probes" {
		t.Fatalf("unexpected report url: %q", cfg.ReportURL)
	}
	if cfg.ProbeIntervalSec != 20 {
		t.Fatalf("unexpected probe interval: %d", cfg.ProbeIntervalSec)
	}
}

func TestLoadFromFileAppliesDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "probe-worker.yml")
	if err := os.WriteFile(path, []byte("worker_id: custom-probe\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.TargetURL != "http://127.0.0.1:8080/master/healthz" {
		t.Fatalf("unexpected default target url: %q", cfg.TargetURL)
	}
	if cfg.ReportURL != "http://127.0.0.1:8090/ingest/api/v1/probes" {
		t.Fatalf("unexpected default report url: %q", cfg.ReportURL)
	}
}
