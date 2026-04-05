package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	MasterAPIURL     string
	IngestGatewayURL string
	Region           string
	Env              string
	Role             string
	TenantCode       string
	ConfigPath       string
	LoopIntervalSec  int
}

func Load() Config {
	configPath := env("AGENT_CONFIG_PATH", defaultConfigPath())
	state := loadState(configPath)
	tenantCode := env("AGENT_TENANT", state.TenantCode)

	return Config{
		MasterAPIURL:     env("MASTER_API_URL", "http://127.0.0.1:8080"),
		IngestGatewayURL: env("INGEST_GATEWAY_URL", "http://127.0.0.1:8090"),
		Region:           env("AGENT_REGION", "local"),
		Env:              env("AGENT_ENV", "dev"),
		Role:             env("AGENT_ROLE", "node"),
		TenantCode:       tenantCode,
		ConfigPath:       configPath,
		LoopIntervalSec:  envInt("AGENT_LOOP_INTERVAL_SEC", 1),
	}
}

type persistedState struct {
	TenantCode string `json:"tenant_code,omitempty"`
}

func SaveTenant(path string, tenantCode string) error {
	if path == "" || tenantCode == "" {
		return nil
	}

	state := loadState(path)
	state.TenantCode = tenantCode

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o600)
}

func loadState(path string) persistedState {
	if path == "" {
		return persistedState{}
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return persistedState{}
	}

	var state persistedState
	if err := json.Unmarshal(body, &state); err != nil {
		return persistedState{}
	}
	return state
}

func defaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return ".gaoming-agent.json"
	}
	return filepath.Join(dir, "gaoming", "agent.json")
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
