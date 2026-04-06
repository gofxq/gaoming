package config

import (
	"os"
	"strconv"
	"strings"
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
		TargetURL:        normalizeProbeTargetURL(env("PROBE_TARGET_URL", "http://127.0.0.1:8080/master/healthz")),
		ReportURL:        normalizeProbeReportURL(env("PROBE_REPORT_URL", "http://127.0.0.1:8090/ingest/api/v1/probes")),
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

func normalizeProbeTargetURL(value string) string {
	value = strings.TrimRight(value, "/")
	if strings.HasSuffix(value, "/master/healthz") || strings.HasSuffix(value, "/ingest/healthz") {
		return value
	}
	if strings.HasSuffix(value, "/healthz") {
		return strings.TrimSuffix(value, "/healthz") + "/master/healthz"
	}
	if strings.HasSuffix(value, "/master") {
		return value + "/healthz"
	}
	return value
}

func normalizeProbeReportURL(value string) string {
	value = strings.TrimRight(value, "/")
	switch {
	case strings.HasSuffix(value, "/ingest/api/v1/probes"):
		return value
	case strings.HasSuffix(value, "/api/v1/probes"):
		return strings.TrimSuffix(value, "/api/v1/probes") + "/ingest/api/v1/probes"
	case strings.HasSuffix(value, "/ingest/api/v1"):
		return value + "/probes"
	case strings.HasSuffix(value, "/ingest"):
		return value + "/api/v1/probes"
	default:
		return value
	}
}
