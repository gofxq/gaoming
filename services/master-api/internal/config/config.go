package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/gofxq/gaoming/pkg/access"
	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "config/master-api.yml"

type Config struct {
	HTTPAddr              string   `yaml:"http_addr"`
	RuntimeBackend        string   `yaml:"runtime_backend"`
	PostgresDSN           string   `yaml:"postgres_dsn"`
	RedisAddr             string   `yaml:"redis_addr"`
	RedisPassword         string   `yaml:"redis_password"`
	RedisDB               int      `yaml:"redis_db"`
	TenantCode            string   `yaml:"tenant_code"`
	TenantName            string   `yaml:"tenant_name"`
	AllowCustomTenantCode bool     `yaml:"allow_custom_tenant_code"`
	SessionCookieName     string   `yaml:"session_cookie_name"`
	SessionSecret         string   `yaml:"session_secret"`
	SessionTTLHours       int      `yaml:"session_ttl_hours"`
	SetupToken            string   `yaml:"setup_token"`
	SessionCookieSecure   bool     `yaml:"session_cookie_secure"`
	AllowedOrigins        []string `yaml:"allowed_origins"`
	DemoTenantCode        string   `yaml:"demo_tenant_code"`
	DemoHostUIDs          []string `yaml:"demo_host_uids"`
	DemoFields            []string `yaml:"demo_fields"`
}

func Load() (Config, error) {
	return LoadFromFile(DefaultConfigFile)
}

func LoadFromFile(path string) (Config, error) {
	cfg := defaultConfig()

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read master-api config %q: %w", path, err)
	}

	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse master-api config %q: %w", path, err)
	}

	cfg.applyDefaults()
	if token := os.Getenv("GAOMING_SETUP_TOKEN"); token != "" {
		cfg.SetupToken = token
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate master-api config %q: %w", path, err)
	}
	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		HTTPAddr:              ":8080",
		RuntimeBackend:        "pg_redis",
		PostgresDSN:           "postgres://gaoming:gaoming@127.0.0.1:5432/gaoming?sslmode=disable",
		RedisAddr:             "127.0.0.1:6379",
		RedisPassword:         "",
		RedisDB:               0,
		TenantCode:            "default",
		TenantName:            "Default Tenant",
		AllowCustomTenantCode: true,
		SessionCookieName:     "gaoming_session",
		SessionSecret:         "change-me",
		SessionTTLHours:       168,
		SessionCookieSecure:   true,
	}
}

func (c *Config) applyDefaults() {
	defaults := defaultConfig()
	if strings.TrimSpace(c.HTTPAddr) == "" {
		c.HTTPAddr = defaults.HTTPAddr
	}
	if strings.TrimSpace(c.RuntimeBackend) == "" {
		c.RuntimeBackend = defaults.RuntimeBackend
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
	if strings.TrimSpace(c.SessionCookieName) == "" {
		c.SessionCookieName = defaults.SessionCookieName
	}
	if strings.TrimSpace(c.SessionSecret) == "" {
		c.SessionSecret = defaults.SessionSecret
	}
	if c.SessionTTLHours <= 0 {
		c.SessionTTLHours = defaults.SessionTTLHours
	}
}

func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.HTTPAddr) == "":
		return fmt.Errorf("http_addr is required")
	case strings.TrimSpace(c.RuntimeBackend) == "":
		return fmt.Errorf("runtime_backend is required")
	case strings.TrimSpace(c.PostgresDSN) == "":
		return fmt.Errorf("postgres_dsn is required")
	case strings.TrimSpace(c.RedisAddr) == "":
		return fmt.Errorf("redis_addr is required")
	case strings.TrimSpace(c.TenantCode) == "":
		return fmt.Errorf("tenant_code is required")
	case strings.TrimSpace(c.TenantName) == "":
		return fmt.Errorf("tenant_name is required")
	case strings.TrimSpace(c.SessionCookieName) == "":
		return fmt.Errorf("session_cookie_name is required")
	case c.SessionTTLHours <= 0:
		return fmt.Errorf("session_ttl_hours must be greater than 0")
	}
	if c.SetupToken != "" && len(c.SetupToken) < 32 {
		return fmt.Errorf("setup_token must be at least 32 bytes")
	}
	for _, origin := range c.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("allowed_origins must contain exact HTTP/HTTPS origins")
		}
	}
	if c.DemoTenantCode != "" || len(c.DemoHostUIDs) > 0 || len(c.DemoFields) > 0 {
		if strings.TrimSpace(c.DemoTenantCode) == "" || len(c.DemoTenantCode) > 64 {
			return fmt.Errorf("demo requires a tenant code")
		}
		if err := access.ValidateShareRequest(access.CreateShareRequest{Title: "Configured demo", HostUIDs: c.DemoHostUIDs, Fields: c.DemoFields, TTLSeconds: 60}); err != nil {
			return fmt.Errorf("invalid demo host or field scope")
		}
	}
	return nil
}
