package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr              string
	GRPCAddr              string
	PostgresDSN           string
	RedisAddr             string
	RedisPassword         string
	RedisDB               int
	TenantCode            string
	TenantName            string
	AllowCustomTenantCode bool
}

func Load() Config {
	return Config{
		HTTPAddr:              env("INGEST_GATEWAY_HTTP_ADDR", ":8090"),
		GRPCAddr:              env("INGEST_GATEWAY_GRPC_ADDR", ":8091"),
		PostgresDSN:           env("INGEST_GATEWAY_POSTGRES_DSN", env("MASTER_API_POSTGRES_DSN", "postgres://gaoming:gaoming@127.0.0.1:5432/gaoming?sslmode=disable")),
		RedisAddr:             env("INGEST_GATEWAY_REDIS_ADDR", env("MASTER_API_REDIS_ADDR", "127.0.0.1:6379")),
		RedisPassword:         env("INGEST_GATEWAY_REDIS_PASSWORD", env("MASTER_API_REDIS_PASSWORD", "")),
		RedisDB:               envInt("INGEST_GATEWAY_REDIS_DB", envInt("MASTER_API_REDIS_DB", 0)),
		TenantCode:            env("INGEST_GATEWAY_TENANT_CODE", env("MASTER_API_TENANT_CODE", "default")),
		TenantName:            env("INGEST_GATEWAY_TENANT_NAME", env("MASTER_API_TENANT_NAME", "Default Tenant")),
		AllowCustomTenantCode: envBool("INGEST_GATEWAY_ALLOW_CUSTOM_TENANT_CODE", envBool("MASTER_API_ALLOW_CUSTOM_TENANT_CODE", true)),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return fallback
}
