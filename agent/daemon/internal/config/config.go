package config

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	MasterAPIURL          string
	IngestGatewayGRPCAddr string
	Region                string
	Env                   string
	Role                  string
	TenantCode            string
	ConfigPath            string
	LoopIntervalSec       int
}

func Load() (Config, error) {
	configPath := os.Getenv("AGENT_CONFIG_PATH")
	if configPath == "" {
		configPath = defaultConfigPath()
	}

	fileState := loadConfigFile(configPath)
	envFile := loadDotEnv(defaultEnvPath())
	fileState.MasterAPIURL = normalizeLegacyURL(fileState.MasterAPIURL, "MASTER_API_URL", "MASTER_API_HTTP_ADDR", envFile)
	masterAPIURL := strings.TrimRight(valueString([]string{"MASTER_API_URL"}, envFile, fileState.MasterAPIURL, "http://127.0.0.1:8080"), "/")
	legacyIngestGatewayURL := strings.TrimRight(valueString([]string{"INGEST_GATEWAY_URL"}, envFile, fileState.LegacyIngestGatewayURL, "http://127.0.0.1:8090"), "/")
	ingestGatewayGRPCAddr := normalizeGRPCAddr(valueString([]string{"INGEST_GATEWAY_GRPC_ADDR"}, envFile, fileState.IngestGatewayGRPCAddr, defaultGRPCAddrForURL(legacyIngestGatewayURL)))

	cfg := Config{
		MasterAPIURL:          masterAPIURL,
		IngestGatewayGRPCAddr: ingestGatewayGRPCAddr,
		Region:                valueString([]string{"AGENT_REGION"}, envFile, fileState.Region, "local"),
		Env:                   valueString([]string{"AGENT_ENV"}, envFile, fileState.Env, "dev"),
		Role:                  valueString([]string{"AGENT_ROLE"}, envFile, fileState.Role, "node"),
		TenantCode:            valueString([]string{"AGENT_TENANT"}, envFile, fileState.TenantCode, ""),
		ConfigPath:            configPath,
		LoopIntervalSec:       valueInt("AGENT_LOOP_INTERVAL_SEC", envFile, fileState.LoopIntervalSec, 1),
	}

	if err := Save(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

type persistedConfig struct {
	MasterAPIURL           string
	LegacyIngestGatewayURL string
	IngestGatewayGRPCAddr  string
	Region                 string
	Env                    string
	Role                   string
	TenantCode             string
	LoopIntervalSec        int
}

func Save(cfg Config) error {
	if cfg.ConfigPath == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cfg.ConfigPath), 0o755); err != nil {
		return err
	}

	body := renderConfigFile(persistedConfig{
		MasterAPIURL:          strings.TrimRight(cfg.MasterAPIURL, "/"),
		IngestGatewayGRPCAddr: cfg.IngestGatewayGRPCAddr,
		Region:                cfg.Region,
		Env:                   cfg.Env,
		Role:                  cfg.Role,
		TenantCode:            cfg.TenantCode,
		LoopIntervalSec:       cfg.LoopIntervalSec,
	})
	return os.WriteFile(cfg.ConfigPath, []byte(body), 0o600)
}

func SaveTenant(path string, tenantCode string) error {
	if path == "" {
		return nil
	}

	state := loadConfigFile(path)
	state.TenantCode = tenantCode
	cfg := Config{
		MasterAPIURL:          state.MasterAPIURL,
		IngestGatewayGRPCAddr: state.IngestGatewayGRPCAddr,
		Region:                state.Region,
		Env:                   state.Env,
		Role:                  state.Role,
		TenantCode:            state.TenantCode,
		ConfigPath:            path,
		LoopIntervalSec:       state.LoopIntervalSec,
	}
	return Save(cfg)
}

func defaultConfigPath() string {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return "agent-config.yaml"
	}
	return filepath.Join(wd, "agent-config.yaml")
}

func defaultEnvPath() string {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return ".env"
	}
	return filepath.Join(wd, ".env")
}

func loadConfigFile(path string) persistedConfig {
	body, err := os.ReadFile(path)
	if err != nil {
		return persistedConfig{}
	}

	var cfg persistedConfig
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		switch key {
		case "master_api_url":
			cfg.MasterAPIURL = value
		case "ingest_gateway_url":
			cfg.LegacyIngestGatewayURL = value
		case "ingest_gateway_grpc_addr":
			cfg.IngestGatewayGRPCAddr = value
		case "region":
			cfg.Region = value
		case "env":
			cfg.Env = value
		case "role":
			cfg.Role = value
		case "tenant_code":
			cfg.TenantCode = value
		case "loop_interval_sec":
			if parsed, err := strconv.Atoi(value); err == nil {
				cfg.LoopIntervalSec = parsed
			}
		}
	}
	return cfg
}

func defaultGRPCAddrForURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return "127.0.0.1:8091"
	}

	host := parsed.Hostname()
	if host == "" {
		return "127.0.0.1:8091"
	}
	return net.JoinHostPort(host, "8091")
}

func normalizeGRPCAddr(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "127.0.0.1:8091"
	}
	return value
}

func loadDotEnv(path string) map[string]string {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	values := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		values[key] = value
	}
	return values
}

func splitKeyValue(line string) (string, string, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	idx := strings.IndexAny(line, "=:")
	if idx <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	value = strings.Trim(value, `"'`)
	return key, value, true
}

func valueString(keys []string, envFile map[string]string, fileValue string, fallback string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
		if value := envFile[key]; value != "" {
			return value
		}
	}
	if fileValue != "" {
		return fileValue
	}
	return fallback
}

func valueInt(key string, envFile map[string]string, fileValue int, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	if value := envFile[key]; value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	if fileValue != 0 {
		return fileValue
	}
	return fallback
}

func renderConfigFile(cfg persistedConfig) string {
	var b strings.Builder
	writeYAMLString(&b, "master_api_url", strings.TrimRight(cfg.MasterAPIURL, "/"))
	writeYAMLString(&b, "ingest_gateway_grpc_addr", normalizeGRPCAddr(cfg.IngestGatewayGRPCAddr))
	writeYAMLString(&b, "region", cfg.Region)
	writeYAMLString(&b, "env", cfg.Env)
	writeYAMLString(&b, "role", cfg.Role)
	writeYAMLString(&b, "tenant_code", cfg.TenantCode)
	fmt.Fprintf(&b, "loop_interval_sec: %d\n", cfg.LoopIntervalSec)
	return b.String()
}

func writeYAMLString(b *strings.Builder, key string, value string) {
	fmt.Fprintf(b, "%s: %s\n", key, strconv.Quote(value))
}

func normalizeLegacyURL(fileValue string, urlKey string, httpAddrKey string, envFile map[string]string) string {
	fileValue = strings.TrimRight(fileValue, "/")
	if fileValue == "" {
		return ""
	}
	if os.Getenv(urlKey) != "" || envFile[urlKey] != "" {
		return fileValue
	}
	legacy := strings.TrimRight(os.Getenv(httpAddrKey), "/")
	if legacy == "" {
		legacy = strings.TrimRight(envFile[httpAddrKey], "/")
	}
	if fileValue == legacy {
		return ""
	}
	return fileValue
}
