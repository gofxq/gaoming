package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr string
}

func Load() Config {
	return Config{
		HTTPAddr: env("MASTER_API_HTTP_ADDR", ":8080"),
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
