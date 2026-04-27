package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "config/master-api.yml"

type Config struct {
	HTTPAddr              string `yaml:"http_addr"`
	RuntimeBackend        string `yaml:"runtime_backend"`
	PostgresDSN           string `yaml:"postgres_dsn"`
	RedisAddr             string `yaml:"redis_addr"`
	RedisPassword         string `yaml:"redis_password"`
	RedisDB               int    `yaml:"redis_db"`
	TenantCode            string `yaml:"tenant_code"`
	TenantName            string `yaml:"tenant_name"`
	AllowCustomTenantCode bool   `yaml:"allow_custom_tenant_code"`
	SessionCookieName     string `yaml:"session_cookie_name"`
	SessionSecret         string `yaml:"session_secret"`
	SessionTTLHours       int    `yaml:"session_ttl_hours"`
	WeChatAppID           string `yaml:"wechat_app_id"`
	WeChatAppSecret       string `yaml:"wechat_app_secret"`
	WeChatRedirectURL     string `yaml:"wechat_redirect_url"`
	WeChatScope           string `yaml:"wechat_scope"`
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
		WeChatAppID:           "",
		WeChatAppSecret:       "",
		WeChatRedirectURL:     "",
		WeChatScope:           "snsapi_login",
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
	if strings.TrimSpace(c.WeChatScope) == "" {
		c.WeChatScope = defaults.WeChatScope
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
	case strings.TrimSpace(c.SessionSecret) == "":
		return fmt.Errorf("session_secret is required")
	case c.SessionTTLHours <= 0:
		return fmt.Errorf("session_ttl_hours must be greater than 0")
	default:
		return nil
	}
}
