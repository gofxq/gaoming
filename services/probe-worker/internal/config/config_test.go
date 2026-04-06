package config

import "testing"

func TestNormalizeProbeTargetURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "http://127.0.0.1:8080/healthz",
			want:  "http://127.0.0.1:8080/master/healthz",
		},
		{
			input: "http://127.0.0.1:8080/master",
			want:  "http://127.0.0.1:8080/master/healthz",
		},
		{
			input: "http://127.0.0.1:8080/master/healthz",
			want:  "http://127.0.0.1:8080/master/healthz",
		},
	}

	for _, tt := range tests {
		if got := normalizeProbeTargetURL(tt.input); got != tt.want {
			t.Fatalf("normalizeProbeTargetURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeProbeReportURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "http://127.0.0.1:8090/api/v1/probes",
			want:  "http://127.0.0.1:8090/ingest/api/v1/probes",
		},
		{
			input: "http://127.0.0.1:8090/ingest",
			want:  "http://127.0.0.1:8090/ingest/api/v1/probes",
		},
		{
			input: "http://127.0.0.1:8090/ingest/api/v1",
			want:  "http://127.0.0.1:8090/ingest/api/v1/probes",
		},
		{
			input: "http://127.0.0.1:8090/ingest/api/v1/probes",
			want:  "http://127.0.0.1:8090/ingest/api/v1/probes",
		},
	}

	for _, tt := range tests {
		if got := normalizeProbeReportURL(tt.input); got != tt.want {
			t.Fatalf("normalizeProbeReportURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
