package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadRequiresAgentConfigYAML(t *testing.T) {
	withTempWorkingDir(t)

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing config file to fail")
	}
	if !strings.Contains(err.Error(), "agent-config.yaml") {
		t.Fatalf("expected error to mention agent-config.yaml, got: %v", err)
	}
}

func TestLoadReadsAgentConfigYAML(t *testing.T) {
	dir := withTempWorkingDir(t)
	writeAgentConfig(t, ""+
		"master_api_url: \"http://saved-master:8080\"\n"+
		"ingest_gateway_grpc_addr: \"saved-ingest:18091\"\n"+
		"region: \"saved-region\"\n"+
		"env: \"saved-env\"\n"+
		"role: \"saved-role\"\n"+
		"tenant_code: \"tenant-saved\"\n"+
		"loop_interval_sec: 9\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.IngestGatewayGRPCAddr != "saved-ingest:18091" {
		t.Fatalf("unexpected ingest grpc addr: %q", cfg.IngestGatewayGRPCAddr)
	}
	if cfg.Region != "saved-region" || cfg.Env != "saved-env" || cfg.Role != "saved-role" {
		t.Fatalf("unexpected host metadata: %+v", cfg)
	}
	if cfg.TenantCode != "tenant-saved" {
		t.Fatalf("unexpected tenant code: %q", cfg.TenantCode)
	}
	if cfg.LoopIntervalSec != 9 {
		t.Fatalf("unexpected loop interval: %d", cfg.LoopIntervalSec)
	}
	expectedPath := filepath.Join(dir, "agent-config.yaml")
	if cfg.ConfigPath != expectedPath {
		t.Fatalf("unexpected config path: %q", cfg.ConfigPath)
	}
}

func TestLoadAppendsDefaultTLSPortWhenGRPCAddrHasNoPort(t *testing.T) {
	withTempWorkingDir(t)
	writeAgentConfig(t, ""+
		"master_api_url: \"http://saved-master:8080\"\n"+
		"ingest_gateway_grpc_addr: \"gm-rpc.gofxq.com\"\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.IngestGatewayGRPCAddr != "gm-rpc.gofxq.com:443" {
		t.Fatalf("unexpected ingest grpc addr: %q", cfg.IngestGatewayGRPCAddr)
	}
}

func TestLoadIgnoresEnvAndDotEnv(t *testing.T) {
	withTempWorkingDir(t)
	writeAgentConfig(t, ""+
		"master_api_url: \"http://from-file:8080\"\n"+
		"ingest_gateway_grpc_addr: \"from-file:18091\"\n"+
		"region: \"file-region\"\n"+
		"env: \"file-env\"\n"+
		"role: \"file-role\"\n"+
		"tenant_code: \"file-tenant\"\n"+
		"loop_interval_sec: 3\n")
	if err := os.WriteFile(".env", []byte("MASTER_API_URL=http://from-dotenv:8080\nAGENT_REGION=dotenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MASTER_API_URL", "http://from-env:8080")
	t.Setenv("INGEST_GATEWAY_GRPC_ADDR", "from-env:19091")
	t.Setenv("AGENT_REGION", "env-region")
	t.Setenv("AGENT_ENV", "env-env")
	t.Setenv("AGENT_ROLE", "env-role")
	t.Setenv("AGENT_TENANT", "env-tenant")
	t.Setenv("AGENT_LOOP_INTERVAL_SEC", "11")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.IngestGatewayGRPCAddr != "from-file:18091" {
		t.Fatalf("unexpected ingest grpc addr: %q", cfg.IngestGatewayGRPCAddr)
	}
	if cfg.Region != "file-region" || cfg.Env != "file-env" || cfg.Role != "file-role" {
		t.Fatalf("unexpected host metadata: %+v", cfg)
	}
	if cfg.TenantCode != "file-tenant" || cfg.LoopIntervalSec != 3 {
		t.Fatalf("unexpected persisted values: %+v", cfg)
	}
}

func TestLoadDerivesGRPCAddrFromLegacyIngestURLInConfigFile(t *testing.T) {
	withTempWorkingDir(t)
	writeAgentConfig(t, ""+
		"master_api_url: \"http://saved-master:8080\"\n"+
		"ingest_gateway_url: \"http://legacy-ingest:8090\"\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.IngestGatewayGRPCAddr != "legacy-ingest:8091" {
		t.Fatalf("unexpected derived ingest grpc addr: %q", cfg.IngestGatewayGRPCAddr)
	}
}

func TestLoadUsesDefaultsForMissingFields(t *testing.T) {
	withTempWorkingDir(t)
	writeAgentConfig(t, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.IngestGatewayGRPCAddr != "127.0.0.1:8091" {
		t.Fatalf("unexpected ingest grpc addr: %q", cfg.IngestGatewayGRPCAddr)
	}
	if cfg.Region != "local" || cfg.Env != "dev" || cfg.Role != "node" {
		t.Fatalf("unexpected host metadata: %+v", cfg)
	}
	if cfg.LoopIntervalSec != 1 {
		t.Fatalf("unexpected loop interval: %d", cfg.LoopIntervalSec)
	}
}

func TestSaveTenantUpdatesAgentConfigYAML(t *testing.T) {
	withTempWorkingDir(t)
	writeAgentConfig(t, ""+
		"master_api_url: \"http://saved-master:8080\"\n"+
		"ingest_gateway_grpc_addr: \"saved-ingest\"\n"+
		"tenant_code: \"\"\n")

	if err := SaveTenant("agent-config.yaml", "tenant-updated"); err != nil {
		t.Fatalf("save tenant: %v", err)
	}

	body, err := os.ReadFile("agent-config.yaml")
	if err != nil {
		t.Fatalf("read agent-config.yaml: %v", err)
	}

	var saved map[string]any
	if err := yaml.Unmarshal(body, &saved); err != nil {
		t.Fatalf("unmarshal saved agent-config.yaml: %v", err)
	}
	if saved["tenant_code"] != "tenant-updated" {
		t.Fatalf("expected updated tenant_code, got %#v", saved["tenant_code"])
	}
	if saved["ingest_gateway_grpc_addr"] != "saved-ingest:443" {
		t.Fatalf("expected normalized ingest addr, got %#v", saved["ingest_gateway_grpc_addr"])
	}
}

func withTempWorkingDir(t *testing.T) string {
	t.Helper()

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
	return dir
}

func writeAgentConfig(t *testing.T, body string) {
	t.Helper()

	if err := os.WriteFile("agent-config.yaml", []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
