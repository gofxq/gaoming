package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReadsDotEnvAndWritesAgentConfigYAML(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	dotenv := "MASTER_API_URL=http://example-master:8080\nINGEST_GATEWAY_URL=http://example-ingest:8090\nAGENT_REGION=prod\nAGENT_ENV=prod\nAGENT_ROLE=edge\nAGENT_LOOP_INTERVAL_SEC=7\n"
	if err := os.WriteFile(".env", []byte(dotenv), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.MasterAPIURL != "http://example-master:8080" {
		t.Fatalf("unexpected master api url: %q", cfg.MasterAPIURL)
	}
	expectedPath := filepath.Join(dir, "agent-config.yaml")
	resolvedExpectedPath, err := filepath.EvalSymlinks(expectedPath)
	if err != nil {
		resolvedExpectedPath = expectedPath
	}
	resolvedConfigPath, err := filepath.EvalSymlinks(cfg.ConfigPath)
	if err != nil {
		resolvedConfigPath = cfg.ConfigPath
	}
	if resolvedConfigPath != resolvedExpectedPath {
		t.Fatalf("unexpected config path: %q", cfg.ConfigPath)
	}

	body, err := os.ReadFile("agent-config.yaml")
	if err != nil {
		t.Fatalf("read agent-config.yaml: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, `master_api_url: "http://example-master:8080"`) {
		t.Fatalf("expected saved config to contain master api url, got:\n%s", text)
	}
	if !strings.Contains(text, "loop_interval_sec: 7") {
		t.Fatalf("expected saved config to contain loop interval, got:\n%s", text)
	}
}

func TestLoadDoesNotTreatServerHTTPAddrAsAgentBaseURL(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	dotenv := "MASTER_API_HTTP_ADDR=https://wrong-master.example.com/\nINGEST_GATEWAY_HTTP_ADDR=https://wrong-ingest.example.com/\n"
	if err := os.WriteFile(".env", []byte(dotenv), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.MasterAPIURL != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected master api url: %q", cfg.MasterAPIURL)
	}
	if cfg.IngestGatewayURL != "http://127.0.0.1:8090" {
		t.Fatalf("unexpected ingest gateway url: %q", cfg.IngestGatewayURL)
	}
}

func TestLoadRepairsLegacyPersistedURLFromServerHTTPAddr(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	dotenv := "MASTER_API_HTTP_ADDR=https://wrong-master.example.com/\nINGEST_GATEWAY_HTTP_ADDR=https://wrong-ingest.example.com/\n"
	if err := os.WriteFile(".env", []byte(dotenv), 0o600); err != nil {
		t.Fatal(err)
	}
	body := "" +
		"master_api_url: \"https://wrong-master.example.com/\"\n" +
		"ingest_gateway_url: \"https://wrong-ingest.example.com/\"\n"
	if err := os.WriteFile("agent-config.yaml", []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.MasterAPIURL != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected repaired master api url: %q", cfg.MasterAPIURL)
	}
	if cfg.IngestGatewayURL != "http://127.0.0.1:8090" {
		t.Fatalf("unexpected repaired ingest gateway url: %q", cfg.IngestGatewayURL)
	}
}

func TestLoadReadsSavedAgentConfigYAMLWithoutDotEnv(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	body := "" +
		"master_api_url: \"http://saved-master:8080\"\n" +
		"ingest_gateway_url: \"http://saved-ingest:8090\"\n" +
		"region: \"saved-region\"\n" +
		"env: \"saved-env\"\n" +
		"role: \"saved-role\"\n" +
		"tenant_code: \"tenant-saved\"\n" +
		"loop_interval_sec: 9\n"
	if err := os.WriteFile("agent-config.yaml", []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.MasterAPIURL != "http://saved-master:8080" {
		t.Fatalf("unexpected master api url: %q", cfg.MasterAPIURL)
	}
	if cfg.TenantCode != "tenant-saved" {
		t.Fatalf("unexpected tenant code: %q", cfg.TenantCode)
	}
	if cfg.LoopIntervalSec != 9 {
		t.Fatalf("unexpected loop interval: %d", cfg.LoopIntervalSec)
	}
}
