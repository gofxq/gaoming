package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "config/probe-worker.yml"

type Config struct {
	WorkerID         string `yaml:"worker_id"`
	TargetURL        string `yaml:"target_url"`
	ReportURL        string `yaml:"report_url"`
	Region           string `yaml:"region"`
	ProbeIntervalSec int    `yaml:"probe_interval_sec"`
}

func Load() (Config, error) {
	return LoadFromFile(DefaultConfigFile)
}

func LoadFromFile(path string) (Config, error) {
	cfg := defaultConfig()

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read probe-worker config %q: %w", path, err)
	}

	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse probe-worker config %q: %w", path, err)
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate probe-worker config %q: %w", path, err)
	}
	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		WorkerID:         "probe-worker-local",
		TargetURL:        "http://127.0.0.1:8080/master/healthz",
		ReportURL:        "http://127.0.0.1:8090/ingest/api/v1/probes",
		Region:           "local",
		ProbeIntervalSec: 15,
	}
}

func (c *Config) applyDefaults() {
	defaults := defaultConfig()
	if strings.TrimSpace(c.WorkerID) == "" {
		c.WorkerID = defaults.WorkerID
	}
	if strings.TrimSpace(c.TargetURL) == "" {
		c.TargetURL = defaults.TargetURL
	}
	if strings.TrimSpace(c.ReportURL) == "" {
		c.ReportURL = defaults.ReportURL
	}
	if strings.TrimSpace(c.Region) == "" {
		c.Region = defaults.Region
	}
	if c.ProbeIntervalSec <= 0 {
		c.ProbeIntervalSec = defaults.ProbeIntervalSec
	}
	c.TargetURL = normalizeProbeTargetURL(c.TargetURL)
	c.ReportURL = normalizeProbeReportURL(c.ReportURL)
}

func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.WorkerID) == "":
		return fmt.Errorf("worker_id is required")
	case strings.TrimSpace(c.TargetURL) == "":
		return fmt.Errorf("target_url is required")
	case strings.TrimSpace(c.ReportURL) == "":
		return fmt.Errorf("report_url is required")
	case strings.TrimSpace(c.Region) == "":
		return fmt.Errorf("region is required")
	case c.ProbeIntervalSec <= 0:
		return fmt.Errorf("probe_interval_sec must be greater than 0")
	default:
		return nil
	}
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
