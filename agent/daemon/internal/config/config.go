package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	IngestGatewayGRPCAddr string
	Region                string
	Env                   string
	Role                  string
	TenantCode            string
	ConfigPath            string
	LoopIntervalSec       int
}

func Load() (Config, error) {
	configPath := defaultConfigPath()

	fileState, err := loadConfigFile(configPath)
	if err != nil {
		return Config{}, err
	}

	ingestGatewayGRPCAddr := strings.TrimSpace(fileState.IngestGatewayGRPCAddr)
	if ingestGatewayGRPCAddr == "" {
		ingestGatewayGRPCAddr = defaultGRPCAddrForURL(fileState.LegacyIngestGatewayURL)
	}
	ingestGatewayGRPCAddr = normalizeGRPCAddr(ingestGatewayGRPCAddr)

	cfg := Config{
		IngestGatewayGRPCAddr: ingestGatewayGRPCAddr,
		Region:                fileString(fileState.Region, "local"),
		Env:                   fileString(fileState.Env, "dev"),
		Role:                  fileString(fileState.Role, "node"),
		TenantCode:            strings.TrimSpace(fileState.TenantCode),
		ConfigPath:            configPath,
		LoopIntervalSec:       fileInt(fileState.LoopIntervalSec, 1),
	}
	return cfg, nil
}

type persistedConfig struct {
	MasterAPIURL           string `yaml:"master_api_url"`
	LegacyIngestGatewayURL string `yaml:"ingest_gateway_url"`
	IngestGatewayGRPCAddr  string `yaml:"ingest_gateway_grpc_addr"`
	Region                 string `yaml:"region"`
	Env                    string `yaml:"env"`
	Role                   string `yaml:"role"`
	TenantCode             string `yaml:"tenant_code"`
	LoopIntervalSec        int    `yaml:"loop_interval_sec"`
}

func Save(cfg Config) error {
	if cfg.ConfigPath == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cfg.ConfigPath), 0o755); err != nil {
		return err
	}

	body, err := renderConfigFile(persistedConfig{
		IngestGatewayGRPCAddr: normalizeGRPCAddr(cfg.IngestGatewayGRPCAddr),
		Region:                cfg.Region,
		Env:                   cfg.Env,
		Role:                  cfg.Role,
		TenantCode:            cfg.TenantCode,
		LoopIntervalSec:       cfg.LoopIntervalSec,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.ConfigPath, []byte(body), 0o600)
}

func SaveTenant(path string, tenantCode string) error {
	if path == "" {
		return nil
	}

	state, err := loadConfigFile(path)
	if err != nil {
		return err
	}
	ingestGatewayGRPCAddr := strings.TrimSpace(state.IngestGatewayGRPCAddr)
	if ingestGatewayGRPCAddr == "" {
		ingestGatewayGRPCAddr = defaultGRPCAddrForURL(state.LegacyIngestGatewayURL)
	}
	state.TenantCode = tenantCode
	cfg := Config{
		IngestGatewayGRPCAddr: normalizeGRPCAddr(ingestGatewayGRPCAddr),
		Region:                fileString(state.Region, "local"),
		Env:                   fileString(state.Env, "dev"),
		Role:                  fileString(state.Role, "node"),
		TenantCode:            state.TenantCode,
		ConfigPath:            path,
		LoopIntervalSec:       fileInt(state.LoopIntervalSec, 1),
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

func loadConfigFile(path string) (persistedConfig, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return persistedConfig{}, fmt.Errorf("read agent config %q: %w", path, err)
	}

	var cfg persistedConfig
	if len(body) == 0 {
		return cfg, nil
	}
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return persistedConfig{}, fmt.Errorf("parse agent config %q: %w", path, err)
	}
	return cfg, nil
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
	if host, port, err := net.SplitHostPort(value); err == nil && host != "" && port != "" {
		return value
	}
	if ip := net.ParseIP(strings.Trim(value, "[]")); ip != nil {
		return net.JoinHostPort(ip.String(), "443")
	}
	if !strings.Contains(value, ":") {
		return net.JoinHostPort(value, "443")
	}
	return value
}

func fileString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func fileInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func renderConfigFile(cfg persistedConfig) (string, error) {
	cfg.MasterAPIURL = strings.TrimRight(cfg.MasterAPIURL, "/")
	cfg.IngestGatewayGRPCAddr = normalizeGRPCAddr(cfg.IngestGatewayGRPCAddr)

	body, err := yaml.Marshal(&cfg)
	if err != nil {
		return "", fmt.Errorf("marshal agent config: %w", err)
	}
	return string(body), nil
}
