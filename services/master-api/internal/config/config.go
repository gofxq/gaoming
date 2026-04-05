package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr       string
	RuntimeBackend string
	PostgresDSN    string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	TenantCode     string
	TenantName     string
}

func Load() Config {
	return Config{
		HTTPAddr:       env("MASTER_API_HTTP_ADDR", ":8080"),
		RuntimeBackend: env("MASTER_API_RUNTIME_BACKEND", "pg_redis"),
		PostgresDSN:    env("MASTER_API_POSTGRES_DSN", "postgres://gaoming:gaoming@127.0.0.1:5432/gaoming?sslmode=disable"),
		RedisAddr:      env("MASTER_API_REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:  env("MASTER_API_REDIS_PASSWORD", ""),
		RedisDB:        envInt("MASTER_API_REDIS_DB", 0),
		TenantCode:     env("MASTER_API_TENANT_CODE", "default"),
		TenantName:     env("MASTER_API_TENANT_NAME", "Default Tenant"),
	}
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
