package http

import (
	"testing"
	"time"

	"github.com/gofxq/gaoming/pkg/state"
)

func TestLatestMetricPointsFromSnapshot(t *testing.T) {
	now := time.Now().UTC()
	snapshot := state.HostSnapshot{
		LastMetricAt:      now,
		CPUUsagePct:       25,
		MemUsedPct:        55,
		MemAvailableBytes: 512,
		SwapUsedPct:       12,
		DiskUsedPct:       66,
		DiskFreeBytes:     1024,
		DiskInodesUsedPct: 17,
		DiskReadBPS:       1024,
		DiskWriteBPS:      2048,
		DiskReadIOPS:      33,
		DiskWriteIOPS:     44,
		Load1:             1.5,
		NetRxBPS:          4096,
		NetTxBPS:          8192,
		NetRxPacketsPS:    80,
		NetTxPacketsPS:    90,
	}

	latest := latestMetricPointsFromSnapshot(snapshot)

	if len(latest) != 16 {
		t.Fatalf("expected 16 latest points, got %d", len(latest))
	}
	if got := latest[state.MetricCPUUsagePct]; got.Value != 25 || !got.TS.Equal(now) {
		t.Fatalf("unexpected cpu latest point: %+v", got)
	}
	if got := latest[state.MetricNetTxBPS]; got.Value != 8192 || !got.TS.Equal(now) {
		t.Fatalf("unexpected net tx latest point: %+v", got)
	}
	if got := latest[state.MetricDiskReadIOPS]; got.Value != 33 || !got.TS.Equal(now) {
		t.Fatalf("unexpected disk read iops latest point: %+v", got)
	}
	if got := latest[state.MetricNetTxPacketsPS]; got.Value != 90 || !got.TS.Equal(now) {
		t.Fatalf("unexpected net tx packets latest point: %+v", got)
	}
}

func TestLatestMetricPointsFromSnapshotWithoutMetrics(t *testing.T) {
	if latest := latestMetricPointsFromSnapshot(state.HostSnapshot{}); latest != nil {
		t.Fatalf("expected nil latest points for zero metric timestamp, got %+v", latest)
	}
}

func TestMatchesTenant(t *testing.T) {
	snapshot := state.HostSnapshot{TenantCode: "tenant-a"}

	if !matchesTenant(snapshot, "") {
		t.Fatal("expected empty tenant filter to match all snapshots")
	}
	if !matchesTenant(snapshot, "tenant-a") {
		t.Fatal("expected tenant filter to match same tenant")
	}
	if matchesTenant(snapshot, "tenant-b") {
		t.Fatal("expected tenant filter to reject different tenant")
	}
}

func TestHostUIDsFromSnapshots(t *testing.T) {
	items := []state.HostSnapshot{
		{HostUID: "host-1"},
		{},
		{HostUID: "host-2"},
	}

	got := hostUIDsFromSnapshots(items)

	if len(got) != 2 {
		t.Fatalf("expected 2 host uids, got %d", len(got))
	}
	if got[0] != "host-1" || got[1] != "host-2" {
		t.Fatalf("unexpected host uids: %+v", got)
	}
}
