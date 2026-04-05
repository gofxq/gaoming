package config

import (
	"os"
	"strconv"
)

type Config struct {
	MasterAPIURL     string
	IngestGatewayURL string
	Region           string
	Env              string
	Role             string
	LoopIntervalSec  int
}

func Load() Config {
	return Config{
		MasterAPIURL:     env("MASTER_API_URL", "http://127.0.0.1:8080"),
		IngestGatewayURL: env("INGEST_GATEWAY_URL", "http://127.0.0.1:8090"),
		Region:           env("AGENT_REGION", "local"),
		Env:              env("AGENT_ENV", "dev"),
		Role:             env("AGENT_ROLE", "node"),
		LoopIntervalSec:  envInt("AGENT_LOOP_INTERVAL_SEC", 5),
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
