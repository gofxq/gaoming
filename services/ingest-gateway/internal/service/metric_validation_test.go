package service

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/gofxq/gaoming/pkg/contracts"
)

func TestValidateMetricBatchAcceptsCollectedMetrics(t *testing.T) {
	req := contracts.PushMetricBatchRequest{
		Points: []contracts.MetricPoint{
			{Name: "host_filesystem_used_pct", Value: 42.5, Labels: map[string]string{"mountpoint": "/srv/data", "device": "/dev/vdb1"}},
		},
		Collectors: []contracts.CollectorStatus{
			{Name: "filesystem", State: contracts.CollectorStateAvailable, MetricNames: []string{"host_filesystem_used_pct"}, DurationMicros: 900},
		},
	}

	if err := validateMetricBatch(req); err != nil {
		t.Fatalf("validate metric batch: %v", err)
	}
}

func TestValidateMetricBatchRejectsUnsafeInput(t *testing.T) {
	tests := []struct {
		name string
		req  contracts.PushMetricBatchRequest
	}{
		{
			name: "non finite value",
			req: contracts.PushMetricBatchRequest{Points: []contracts.MetricPoint{
				{Name: "host_cpu_usage_pct", Value: math.Inf(1)},
			}},
		},
		{
			name: "oversized label",
			req: contracts.PushMetricBatchRequest{Points: []contracts.MetricPoint{
				{Name: "host_filesystem_used_pct", Labels: map[string]string{"mountpoint": strings.Repeat("a", maxLabelValueBytes+1)}},
			}},
		},
		{
			name: "control character",
			req: contracts.PushMetricBatchRequest{Points: []contracts.MetricPoint{
				{Name: "host_net_bytes_total", Labels: map[string]string{"interface": "eth0\nforged"}},
			}},
		},
		{
			name: "invalid collector state",
			req: contracts.PushMetricBatchRequest{Collectors: []contracts.CollectorStatus{
				{Name: "cpu", State: contracts.CollectorState(99)},
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateMetricBatch(test.req)
			if !errors.Is(err, ErrInvalidMetricBatch) {
				t.Fatalf("expected invalid metric batch, got %v", err)
			}
		})
	}
}
