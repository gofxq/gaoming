package config

import "os"

type Config struct {
	HTTPAddr string
	GRPCAddr string
}

func Load() Config {
	return Config{
		HTTPAddr: env("INGEST_GATEWAY_HTTP_ADDR", ":8090"),
		GRPCAddr: env("INGEST_GATEWAY_GRPC_ADDR", ":8091"),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
