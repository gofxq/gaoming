package service

import (
	"testing"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
)

type numericFixture struct {
	UsedPercent float64 `json:"usedPercent"`
	ReadBytes   uint64  `json:"readBytes"`
	Ignored     string  `json:"ignored"`
}

func TestAppendNumericFieldsAdaptsExportedNumericValues(t *testing.T) {
	points := appendNumericFields(nil, "test_", numericFixture{
		UsedPercent: 37.5,
		ReadBytes:   1024,
		Ignored:     "secret",
	}, map[string]string{"device": "sda"}, nil)

	if len(points) != 2 {
		t.Fatalf("expected two numeric points, got %d", len(points))
	}
	if points[0].Name != "test_used_percent" || points[0].Value != 37.5 {
		t.Fatalf("unexpected first point: %+v", points[0])
	}
	if points[1].Name != "test_read_bytes" || points[1].Labels["device"] != "sda" {
		t.Fatalf("unexpected second point: %+v", points[1])
	}
}

func TestAppendMetricPointsDeduplicatesOnlyMatchingLabelSet(t *testing.T) {
	now := time.Now().UTC()
	points := appendMetricPoints([]contracts.MetricPoint{{Name: "metric", TS: now}}, []contracts.MetricPoint{
		{Name: "metric"},
		{Name: "metric", Labels: map[string]string{"cpu": "0"}},
	}, now)

	if len(points) != 2 {
		t.Fatalf("expected aggregate and labeled point, got %d", len(points))
	}
	if !points[1].TS.Equal(now) {
		t.Fatalf("expected missing timestamp to be populated, got %v", points[1].TS)
	}
}

func TestGopsutilSamplerReportsEveryCollectorStatus(t *testing.T) {
	startedAt := time.Now()
	metrics := (&gopsutilSystemSampler{rootPath: systemRootPath()}).Sample(time.Now().UTC())
	elapsed := time.Since(startedAt)

	if elapsed > 100*time.Millisecond {
		t.Fatalf("expected collection to finish within 100ms, took %s", elapsed)
	}
	if len(metrics.Collectors) != 9 {
		t.Fatalf("expected status for all 9 collectors, got %d", len(metrics.Collectors))
	}
	for _, collector := range metrics.Collectors {
		if collector.Name == "" || collector.State == contracts.CollectorStateUnspecified {
			t.Fatalf("invalid collector status: %+v", collector)
		}
		if collector.State != contracts.CollectorStateUnavailable && len(collector.MetricNames) == 0 {
			t.Fatalf("available collector %q did not report metric names", collector.Name)
		}
	}
	if metrics.Structured == nil || metrics.Structured.Cpu == nil || metrics.Structured.Memory == nil {
		t.Fatalf("expected structured CPU and memory metrics, got %+v", metrics.Structured)
	}
}

func TestCalculateRateInitialSampleReturnsZero(t *testing.T) {
	now := time.Now().UTC()

	a, b, curA, curB := calculateRate(0, 0, 100, 200, time.Time{}, now)

	if a != 0 || b != 0 {
		t.Fatalf("expected zero rates for initial sample, got %d and %d", a, b)
	}
	if curA != 100 || curB != 200 {
		t.Fatalf("expected current counters to pass through, got %d and %d", curA, curB)
	}
}

func TestCalculateRateCounterResetReturnsZero(t *testing.T) {
	prevAt := time.Now().UTC()
	now := prevAt.Add(2 * time.Second)

	a, b, curA, curB := calculateRate(200, 300, 100, 150, prevAt, now)

	if a != 0 || b != 0 {
		t.Fatalf("expected zero rates after counter reset, got %d and %d", a, b)
	}
	if curA != 100 || curB != 150 {
		t.Fatalf("expected reset counters to pass through, got %d and %d", curA, curB)
	}
}

func TestCalculateRateNormalDelta(t *testing.T) {
	prevAt := time.Now().UTC()
	now := prevAt.Add(2 * time.Second)

	a, b, _, _ := calculateRate(100, 40, 300, 100, prevAt, now)

	if a != 100 || b != 30 {
		t.Fatalf("expected rates 100 and 30, got %d and %d", a, b)
	}
}
