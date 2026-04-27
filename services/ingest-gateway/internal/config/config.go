package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "config/ingest-gateway.yml"

type Config struct {
	HTTPAddr              string `yaml:"http_addr"`
	GRPCAddr              string `yaml:"grpc_addr"`
	PostgresDSN           string `yaml:"postgres_dsn"`
	RedisAddr             string `yaml:"redis_addr"`
	RedisPassword         string `yaml:"redis_password"`
	RedisDB               int    `yaml:"redis_db"`
	TenantCode            string `yaml:"tenant_code"`
	TenantName            string `yaml:"tenant_name"`
	AllowCustomTenantCode bool   `yaml:"allow_custom_tenant_code"`
}

func Load() (Config, error) {
	return LoadFromFile(DefaultConfigFile)
}

func LoadFromFile(path string) (Config, error) {
	cfg := defaultConfig()

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read ingest-gateway config %q: %w", path, err)
	}

	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse ingest-gateway config %q: %w", path, err)
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate ingest-gateway config %q: %w", path, err)
	}
	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		HTTPAddr:              ":8090",
		GRPCAddr:              ":8091",
		PostgresDSN:           "postgres://gaoming:gaoming@127.0.0.1:5432/gaoming?sslmode=disable",
		RedisAddr:             "127.0.0.1:6379",
		RedisPassword:         "",
		RedisDB:               0,
		TenantCode:            "default",
		TenantName:            "Default Tenant",
		AllowCustomTenantCode: true,
	}
}

func (c *Config) applyDefaults() {
	defaults := defaultConfig()
	if strings.TrimSpace(c.HTTPAddr) == "" {
		c.HTTPAddr = defaults.HTTPAddr
	}
	if strings.TrimSpace(c.GRPCAddr) == "" {
		c.GRPCAddr = defaults.GRPCAddr
	}
	if strings.TrimSpace(c.PostgresDSN) == "" {
		c.PostgresDSN = defaults.PostgresDSN
	}
	if strings.TrimSpace(c.RedisAddr) == "" {
		c.RedisAddr = defaults.RedisAddr
	}
	if strings.TrimSpace(c.TenantCode) == "" {
		c.TenantCode = defaults.TenantCode
	}
	if strings.TrimSpace(c.TenantName) == "" {
		c.TenantName = defaults.TenantName
	}
}

func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.HTTPAddr) == "":
		return fmt.Errorf("http_addr is required")
	case strings.TrimSpace(c.GRPCAddr) == "":
		return fmt.Errorf("grpc_addr is required")
	case strings.TrimSpace(c.PostgresDSN) == "":
		return fmt.Errorf("postgres_dsn is required")
	case strings.TrimSpace(c.RedisAddr) == "":
		return fmt.Errorf("redis_addr is required")
	case strings.TrimSpace(c.TenantCode) == "":
		return fmt.Errorf("tenant_code is required")
	case strings.TrimSpace(c.TenantName) == "":
		return fmt.Errorf("tenant_name is required")
	default:
		return nil
	}
}
