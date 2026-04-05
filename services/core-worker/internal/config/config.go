package config

import (
	"os"
	"strconv"
)

type Config struct {
	LoopIntervalSec int
}

func Load() Config {
	return Config{
		LoopIntervalSec: envInt("CORE_WORKER_LOOP_INTERVAL_SEC", 15),
	}
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
