package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"

	masterconfig "github.com/gofxq/gaoming/server/internal/master/config"
	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "config/server.yml"

type Config struct {
	Master           masterconfig.Config `yaml:"-"`
	GRPCAddr         string              `yaml:"grpc_addr"`
	CoreIntervalSec  int                 `yaml:"core_interval_sec"`
	ProbeIntervalSec int                 `yaml:"probe_interval_sec"`
	ProbeTargetURL   string              `yaml:"probe_target_url"`
	RedisNamespace   string              `yaml:"redis_namespace"`
}

func Load(path string) (Config, error) {
	master, err := masterconfig.LoadFromFile(path)
	if err != nil {
		return Config{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{Master: master, GRPCAddr: "0.0.0.0:8091", CoreIntervalSec: 15, ProbeIntervalSec: 15}
	// Unlike legacy wildcard defaults, the unified defaults explicitly bind IPv4.
	var listen struct {
		HTTPAddr string `yaml:"http_addr"`
	}
	if err := yaml.Unmarshal(raw, &listen); err != nil {
		return Config{}, fmt.Errorf("parse server config: %w", err)
	}
	if listen.HTTPAddr == "" {
		cfg.Master.HTTPAddr = "0.0.0.0:8080"
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse server config: %w", err)
	}
	if addr := os.Getenv("GAOMING_HTTP_ADDR"); addr != "" {
		cfg.Master.HTTPAddr = addr
	}
	if addr := os.Getenv("GAOMING_GRPC_ADDR"); addr != "" {
		cfg.GRPCAddr = addr
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if err := c.Master.Validate(); err != nil {
		return err
	}
	if c.Master.RuntimeBackend != "pg_redis" {
		return fmt.Errorf("unified server requires runtime_backend: pg_redis")
	}
	for name, addr := range map[string]string{"http_addr": c.Master.HTTPAddr, "grpc_addr": c.GRPCAddr} {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("invalid %s: expected host:port", name)
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 0 || n > 65535 {
			return fmt.Errorf("invalid %s port", name)
		}
	}
	if c.CoreIntervalSec <= 0 || c.ProbeIntervalSec <= 0 {
		return fmt.Errorf("worker intervals must be positive")
	}
	if c.ProbeTargetURL != "" {
		u, err := url.Parse(c.ProbeTargetURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			return fmt.Errorf("probe_target_url must be an HTTP(S) URL without credentials")
		}
	}
	return nil
}
