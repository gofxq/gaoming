package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "config/core-worker.yml"

type Config struct {
	LoopIntervalSec int `yaml:"loop_interval_sec"`
}

func Load() (Config, error) {
	return LoadFromFile(DefaultConfigFile)
}

func LoadFromFile(path string) (Config, error) {
	cfg := defaultConfig()

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read core-worker config %q: %w", path, err)
	}

	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse core-worker config %q: %w", path, err)
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate core-worker config %q: %w", path, err)
	}
	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		LoopIntervalSec: 15,
	}
}

func (c *Config) applyDefaults() {
	if c.LoopIntervalSec <= 0 {
		c.LoopIntervalSec = defaultConfig().LoopIntervalSec
	}
}

func (c Config) Validate() error {
	if c.LoopIntervalSec <= 0 {
		return fmt.Errorf("loop_interval_sec must be greater than 0")
	}
	return nil
}
