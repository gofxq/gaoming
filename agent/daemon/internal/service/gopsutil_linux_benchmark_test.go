//go:build linux

package service

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/docker"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"
)

type optionalProbeResult struct {
	entities int
	errors   int
}

// BenchmarkGopsutilOptionalGroups measures capabilities deliberately excluded
// from the default Agent path. It never logs the values returned by sensitive
// APIs; only timings, entity counts, and error counts reach benchmark output.
func BenchmarkGopsutilOptionalGroups(b *testing.B) {
	probes := []struct {
		name string
		run  func(context.Context) optionalProbeResult
	}{
		{name: "host_users", run: probeHostUsers},
		{name: "memory_extended", run: probeExtendedMemory},
		{name: "swap_devices", run: probeSwapDevices},
		{name: "filesystem_partitions_all", run: probeAllPartitions},
		{name: "filesystem_usage_all", run: probeAllFilesystemUsage},
		{name: "disk_identifiers", run: probeDiskIdentifiers},
		{name: "network_connections_all", run: probeNetworkConnections},
		{name: "process_inventory", run: probeProcessInventory},
		{name: "process_numeric_sequential", run: probeProcessNumericSequential},
		{name: "process_numeric_bounded_parallel", run: probeProcessNumericParallel},
		{name: "process_threads", run: probeProcessThreads},
		{name: "sensor_extended", run: probeExtendedSensors},
		{name: "docker_inventory", run: probeDockerInventory},
		{name: "docker_cgroup_stats", run: probeDockerCgroupStats},
	}

	for _, probe := range probes {
		b.Run(probe.name, func(b *testing.B) {
			b.ReportAllocs()
			var entities, failures int
			for b.Loop() {
				result := probe.run(context.Background())
				entities += result.entities
				failures += result.errors
			}
			b.ReportMetric(float64(entities)/float64(b.N), "entities/op")
			b.ReportMetric(float64(failures)/float64(b.N), "errors/op")
		})
	}
}

func probeHostUsers(ctx context.Context) optionalProbeResult {
	values, err := host.UsersWithContext(ctx)
	return optionalResult(len(values), err)
}

func probeExtendedMemory(ctx context.Context) optionalProbeResult {
	_, err := mem.NewExLinux().VirtualMemoryWithContext(ctx)
	return optionalResult(1, err)
}

func probeSwapDevices(ctx context.Context) optionalProbeResult {
	values, err := mem.SwapDevicesWithContext(ctx)
	return optionalResult(len(values), err)
}

func probeAllPartitions(ctx context.Context) optionalProbeResult {
	values, err := disk.PartitionsWithContext(ctx, true)
	return optionalResult(len(values), err)
}

func probeAllFilesystemUsage(ctx context.Context) optionalProbeResult {
	partitions, err := disk.PartitionsWithContext(ctx, true)
	result := optionalResult(0, err)
	seen := make(map[string]struct{}, len(partitions))
	for _, partition := range partitions {
		if _, ok := seen[partition.Mountpoint]; ok {
			continue
		}
		seen[partition.Mountpoint] = struct{}{}
		if _, err := disk.UsageWithContext(ctx, partition.Mountpoint); err != nil {
			result.errors++
			continue
		}
		result.entities++
	}
	return result
}

func probeDiskIdentifiers(ctx context.Context) optionalProbeResult {
	values, err := disk.IOCountersWithContext(ctx)
	result := optionalResult(len(values), err)
	for name := range values {
		if _, err := disk.SerialNumberWithContext(ctx, name); err != nil {
			result.errors++
		}
		if _, err := disk.LabelWithContext(ctx, name); err != nil {
			result.errors++
		}
	}
	return result
}

func probeNetworkConnections(ctx context.Context) optionalProbeResult {
	values, err := gnet.ConnectionsWithoutUidsWithContext(ctx, "all")
	return optionalResult(len(values), err)
}

func probeProcessInventory(ctx context.Context) optionalProbeResult {
	values, err := process.ProcessesWithContext(ctx)
	return optionalResult(len(values), err)
}

func probeProcessNumericSequential(ctx context.Context) optionalProbeResult {
	processes, err := process.ProcessesWithContext(ctx)
	result := optionalResult(len(processes), err)
	for _, item := range processes {
		result.errors += probeProcessNumeric(ctx, item)
	}
	return result
}

func probeProcessNumericParallel(ctx context.Context) optionalProbeResult {
	processes, err := process.ProcessesWithContext(ctx)
	result := optionalResult(len(processes), err)
	if len(processes) == 0 {
		return result
	}

	workers := runtime.GOMAXPROCS(0)
	if workers > len(processes) {
		workers = len(processes)
	}
	jobs := make(chan *process.Process)
	var failures atomic.Int64
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for item := range jobs {
				failures.Add(int64(probeProcessNumeric(ctx, item)))
			}
		}()
	}
	for _, item := range processes {
		jobs <- item
	}
	close(jobs)
	group.Wait()
	result.errors += int(failures.Load())
	return result
}

func probeProcessNumeric(ctx context.Context, item *process.Process) int {
	var failures int
	if _, err := item.TimesWithContext(ctx); err != nil {
		failures++
	}
	if _, err := item.MemoryInfoWithContext(ctx); err != nil {
		failures++
	}
	if _, err := item.IOCountersWithContext(ctx); err != nil {
		failures++
	}
	if _, err := item.NumCtxSwitchesWithContext(ctx); err != nil {
		failures++
	}
	if _, err := item.NumFDsWithContext(ctx); err != nil {
		failures++
	}
	if _, err := item.NumThreadsWithContext(ctx); err != nil {
		failures++
	}
	if _, err := item.PageFaultsWithContext(ctx); err != nil {
		failures++
	}
	return failures
}

func probeProcessThreads(ctx context.Context) optionalProbeResult {
	processes, err := process.ProcessesWithContext(ctx)
	result := optionalResult(0, err)
	for _, item := range processes {
		threads, err := item.ThreadsWithContext(ctx)
		if err != nil {
			result.errors++
			continue
		}
		result.entities += len(threads)
	}
	return result
}

func probeExtendedSensors(ctx context.Context) optionalProbeResult {
	values, err := sensors.NewExLinux().TemperatureWithContext(ctx)
	return optionalResult(len(values), err)
}

func probeDockerInventory(ctx context.Context) optionalProbeResult {
	values, err := docker.GetDockerStatWithContext(ctx)
	return optionalResult(len(values), err)
}

func probeDockerCgroupStats(ctx context.Context) optionalProbeResult {
	ids, err := docker.GetDockerIDListWithContext(ctx)
	result := optionalResult(len(ids), err)
	for _, id := range ids {
		if _, err := docker.CgroupCPUDockerWithContext(ctx, id); err != nil {
			result.errors++
		}
		if _, err := docker.CgroupMemDockerWithContext(ctx, id); err != nil {
			result.errors++
		}
	}
	return result
}

func optionalResult(entities int, err error) optionalProbeResult {
	result := optionalProbeResult{entities: entities}
	if err != nil {
		result.errors = 1
	}
	return result
}
