package service

import (
	"context"
	"testing"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
	"google.golang.org/protobuf/proto"
)

var benchmarkMetricCollectors = []struct {
	name string
	run  func(context.Context) collectorResult
}{
	{name: "host", run: collectHostMetrics},
	{name: "cpu", run: collectCPUMetrics},
	{name: "memory", run: collectMemoryMetrics},
	{name: "swap", run: collectSwapMetrics},
	{name: "load", run: collectLoadMetrics},
	{name: "filesystem", run: collectFilesystemMetrics},
	{name: "disk_io", run: collectDiskIOMetrics},
	{name: "network", run: collectNetworkMetrics},
	{name: "sensors", run: collectSensorMetrics},
}

// BenchmarkSystemMetricCollectors profiles the exact collectors used by the
// Agent. Run it on the target host because point counts and timings are tied to
// the machine's CPUs, mounts, block devices, interfaces, and sensors.
func BenchmarkSystemMetricCollectors(b *testing.B) {
	for _, collector := range benchmarkMetricCollectors {
		b.Run(collector.name, func(b *testing.B) {
			b.ReportAllocs()
			var points, names, failures int
			for b.Loop() {
				result := collector.run(context.Background())
				points += len(result.points)
				names += len(uniqueMetricNames(result.points))
				failures += result.failures
			}
			b.ReportMetric(float64(points)/float64(b.N), "points/op")
			b.ReportMetric(float64(names)/float64(b.N), "names/op")
			b.ReportMetric(float64(failures)/float64(b.N), "errors/op")
		})
	}

	b.Run("all_parallel_95ms", func(b *testing.B) {
		b.ReportAllocs()
		var points, names, degraded, unavailable, timedOut int
		for b.Loop() {
			ctx, cancel := context.WithTimeout(context.Background(), maxCollectionDuration)
			collected, statuses, _, _ := collectExtendedMetrics(ctx)
			cancel()
			points += len(collected)
			names += len(uniqueMetricNames(collected))
			for _, status := range statuses {
				switch status.State {
				case contracts.CollectorStateUnavailable:
					unavailable++
				case contracts.CollectorStateDegraded:
					degraded++
				}
				if status.TimedOut {
					timedOut++
				}
			}
		}
		b.ReportMetric(float64(points)/float64(b.N), "points/op")
		b.ReportMetric(float64(names)/float64(b.N), "names/op")
		b.ReportMetric(float64(degraded)/float64(b.N), "degraded/op")
		b.ReportMetric(float64(unavailable)/float64(b.N), "unavailable/op")
		b.ReportMetric(float64(timedOut)/float64(b.N), "timeouts/op")
	})

	b.Run("agent_sample_95ms", func(b *testing.B) {
		b.ReportAllocs()
		sampler := &gopsutilSystemSampler{rootPath: systemRootPath()}
		var points int
		for b.Loop() {
			metrics := sampler.Sample(time.Now().UTC())
			points += len(metrics.ExtraPoints)
		}
		b.ReportMetric(float64(points)/float64(b.N), "points/op")
	})
}

func BenchmarkSystemMetricBatchEncoding(b *testing.B) {
	now := time.Now().UTC()
	sampler := &gopsutilSystemSampler{rootPath: systemRootPath()}
	metrics := sampler.Sample(now)
	agent := &Agent{metricSeq: 1}
	digest := agent.digestFromMetrics(metrics)
	points := appendMetricPoints(benchmarkDigestPoints(now, digest), metrics.ExtraPoints, now)
	request := toProtoPushMetricBatchRequest(contracts.PushMetricBatchRequest{
		HostUID:     "benchmark-host",
		AgentID:     "benchmark-agent",
		BatchSeq:    1,
		CollectedAt: now,
		Points:      points,
		Collectors:  metrics.Collectors,
		Digest:      digest,
	})
	request.SystemMetrics = metrics.Structured

	b.ReportAllocs()
	var encoded []byte
	for b.Loop() {
		encoded, _ = proto.MarshalOptions{}.MarshalAppend(encoded[:0], request)
	}
	b.ReportMetric(float64(len(points)), "points/op")
	b.ReportMetric(float64(len(uniqueMetricNames(points))), "names/op")
	b.ReportMetric(float64(proto.Size(request)), "proto_bytes/op")
	if len(encoded) == 0 {
		b.Fatal("expected a non-empty protobuf batch")
	}
}

func benchmarkDigestPoints(now time.Time, digest contracts.AgentDigest) []contracts.MetricPoint {
	return []contracts.MetricPoint{
		{Name: "runtime_uptime_seconds", TS: now},
		{Name: "host_cpu_usage_pct", Value: digest.CPUUsagePct, TS: now},
		{Name: "host_mem_used_pct", Value: digest.MemUsedPct, TS: now},
		{Name: "host_mem_available_bytes", Value: float64(digest.MemAvailableBytes), TS: now},
		{Name: "host_swap_used_pct", Value: digest.SwapUsedPct, TS: now},
		{Name: "host_disk_used_pct", Value: digest.DiskUsedPct, TS: now},
		{Name: "host_disk_free_bytes", Value: float64(digest.DiskFreeBytes), TS: now},
		{Name: "host_disk_inodes_used_pct", Value: digest.DiskInodesUsedPct, TS: now},
		{Name: "host_disk_read_bps", Value: float64(digest.DiskReadBPS), TS: now},
		{Name: "host_disk_write_bps", Value: float64(digest.DiskWriteBPS), TS: now},
		{Name: "host_disk_read_iops", Value: float64(digest.DiskReadIOPS), TS: now},
		{Name: "host_disk_write_iops", Value: float64(digest.DiskWriteIOPS), TS: now},
		{Name: "host_load1", Value: digest.Load1, TS: now},
		{Name: "host_net_rx_bps", Value: float64(digest.NetRxBPS), TS: now},
		{Name: "host_net_tx_bps", Value: float64(digest.NetTxBPS), TS: now},
		{Name: "host_net_rx_packets_ps", Value: float64(digest.NetRxPacketsPS), TS: now},
		{Name: "host_net_tx_packets_ps", Value: float64(digest.NetTxPacketsPS), TS: now},
	}
}
