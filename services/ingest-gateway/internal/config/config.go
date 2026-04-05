package config

import "os"

type Config struct {
	HTTPAddr string
}

func Load() Config {
	return Config{
		HTTPAddr: env("INGEST_GATEWAY_HTTP_ADDR", ":8090"),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
