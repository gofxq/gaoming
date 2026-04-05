package config

import (
	"os"
	"strconv"
)

type Config struct {
	WorkerID         string
	TargetURL        string
	ReportURL        string
	Region           string
	ProbeIntervalSec int
}

func Load() Config {
	return Config{
		WorkerID:         env("PROBE_WORKER_ID", "probe-worker-local"),
		TargetURL:        env("PROBE_TARGET_URL", "http://127.0.0.1:8080/healthz"),
		ReportURL:        env("PROBE_REPORT_URL", "http://127.0.0.1:8090/api/v1/probes"),
		Region:           env("PROBE_REGION", "local"),
		ProbeIntervalSec: envInt("PROBE_INTERVAL_SEC", 15),
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
