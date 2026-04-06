package service

import "testing"

func TestMasterAPIURL(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		endpoint string
		want     string
	}{
		{
			name:     "root base",
			base:     "http://127.0.0.1:8080",
			endpoint: "agents/register",
			want:     "http://127.0.0.1:8080/master/api/v1/agents/register",
		},
		{
			name:     "service base",
			base:     "http://127.0.0.1:8080/master",
			endpoint: "agents/register",
			want:     "http://127.0.0.1:8080/master/api/v1/agents/register",
		},
		{
			name:     "api base",
			base:     "http://127.0.0.1:8080/master/api/v1",
			endpoint: "agents/register",
			want:     "http://127.0.0.1:8080/master/api/v1/agents/register",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := masterAPIURL(tt.base, tt.endpoint); got != tt.want {
				t.Fatalf("masterAPIURL(%q, %q) = %q, want %q", tt.base, tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestIngestAPIURL(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		endpoint string
		want     string
	}{
		{
			name:     "root base",
			base:     "http://127.0.0.1:8090",
			endpoint: "metrics",
			want:     "http://127.0.0.1:8090/ingest/api/v1/metrics",
		},
		{
			name:     "service base",
			base:     "http://127.0.0.1:8090/ingest",
			endpoint: "metrics",
			want:     "http://127.0.0.1:8090/ingest/api/v1/metrics",
		},
		{
			name:     "api base",
			base:     "http://127.0.0.1:8090/ingest/api/v1",
			endpoint: "metrics",
			want:     "http://127.0.0.1:8090/ingest/api/v1/metrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ingestAPIURL(tt.base, tt.endpoint); got != tt.want {
				t.Fatalf("ingestAPIURL(%q, %q) = %q, want %q", tt.base, tt.endpoint, got, tt.want)
			}
		})
	}
}
